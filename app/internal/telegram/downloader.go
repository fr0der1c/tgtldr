package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/gotd/td/telegram/downloader"
	messagepeer "github.com/gotd/td/telegram/message/peer"
	dialogsquery "github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"golang.org/x/sync/errgroup"
)

const (
	mediaDownloadLimit = int64(100 * 1024 * 1024)
	mediaRetryLimit    = 5
	mediaPollInterval  = 2 * time.Second
)

// runDownloadWorker 持续领取当前账号的媒体和头像任务，直到监听上下文结束。
func (s *Service) runDownloadWorker(ctx context.Context, accountID int64, api *tg.Client) error {
	if err := s.store.Assets.RecoverInterrupted(ctx, accountID); err != nil {
		return err
	}
	d := downloader.NewDownloader()
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		if err := s.reconcileStoredMedia(groupCtx, accountID); err != nil {
			log.Printf("telegram stored media reconciliation failed: account_id=%d error=%v", accountID, err)
		}
		if err := s.reconcileDialogAvatars(groupCtx, accountID, api); err != nil {
			log.Printf("telegram chat avatar reconciliation failed: account_id=%d error=%v", accountID, err)
		}
		if err := s.reconcileStoredEntities(groupCtx, accountID, api); err != nil {
			log.Printf("telegram sender avatar reconciliation failed: account_id=%d error=%v", accountID, err)
		}
		return nil
	})
	group.Go(func() error {
		return s.runDownloadQueue(groupCtx, accountID, api, d)
	})
	return group.Wait()
}

// runDownloadQueue 轮询持久队列并处理资源，单个资源失败不会中断账号监听。
func (s *Service) runDownloadQueue(ctx context.Context, accountID int64, api *tg.Client, d *downloader.Downloader) error {
	for {
		asset, err := s.store.Assets.ClaimNext(ctx, accountID)
		if err != nil {
			return fmt.Errorf("claim media download for account %d: %w", accountID, err)
		}
		if asset == nil {
			if err := waitMediaPoll(ctx); err != nil {
				return err
			}
			continue
		}
		if err := s.downloadAsset(ctx, accountID, api, d, *asset); err != nil {
			retry := asset.RetryCount+1 < mediaRetryLimit &&
				!errors.Is(err, errMediaTooLarge) && !errors.Is(err, errMediaUnavailable)
			delay := mediaRetryDelay(asset.RetryCount)
			if storeErr := s.store.Assets.Fail(ctx, asset.ID, err.Error(), retry, delay); storeErr != nil {
				return storeErr
			}
			log.Printf("telegram media download failed: asset_id=%d account_id=%d retry=%t error=%v", asset.ID, accountID, retry, err)
		}
	}
}

// reconcileStoredEntities 分批重新获取旧消息，补齐发言人 access hash、头像和新文件引用。
func (s *Service) reconcileStoredEntities(ctx context.Context, accountID int64, api *tg.Client) error {
	const batchSize = 100
	lastID := int64(0)
	for {
		candidates, err := s.store.Messages.ListMissingSenderEntities(ctx, accountID, lastID, batchSize)
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			return nil
		}
		groups := make(map[int64][]store.MessageEntityCandidate)
		chatOrder := make([]int64, 0)
		for _, candidate := range candidates {
			lastID = candidate.MessageID
			if _, exists := groups[candidate.ChatID]; !exists {
				chatOrder = append(chatOrder, candidate.ChatID)
			}
			groups[candidate.ChatID] = append(groups[candidate.ChatID], candidate)
		}
		for _, chatID := range chatOrder {
			group := groups[chatID]
			if err := s.reconcileStoredEntityGroupWithWait(ctx, accountID, api, group); err != nil {
				log.Printf("skip telegram sender entity batch: chat_id=%d error=%v", group[0].ChatID, err)
			}
		}
	}
}

// reconcileStoredEntityGroupWithWait 遇到 Telegram 限流时按服务端指定时间等待并重试当前批次。
func (s *Service) reconcileStoredEntityGroupWithWait(ctx context.Context, accountID int64, api *tg.Client, candidates []store.MessageEntityCandidate) error {
	for {
		err := s.reconcileStoredEntityGroup(ctx, accountID, api, candidates)
		if err == nil {
			return nil
		}
		var floodErr *FloodWaitError
		if !errors.As(wrapTelegramError(err), &floodErr) {
			return err
		}
		log.Printf("telegram sender avatar reconciliation waiting: account_id=%d chat_id=%d wait=%s", accountID, candidates[0].ChatID, floodErr.Wait)
		timer := time.NewTimer(floodErr.Wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// reconcileStoredEntityGroup 使用一次 Telegram 请求补齐同一群组的一批消息。
func (s *Service) reconcileStoredEntityGroup(ctx context.Context, accountID int64, api *tg.Client, candidates []store.MessageEntityCandidate) error {
	ids := make([]tg.InputMessageClass, 0, len(candidates))
	byTelegramID := make(map[int]store.MessageEntityCandidate, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, &tg.InputMessageID{ID: candidate.TelegramMessageID})
		byTelegramID[candidate.TelegramMessageID] = candidate
	}
	var result tg.MessagesMessagesClass
	var err error
	if candidates[0].ChatType == "supergroup" {
		result, err = api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: candidates[0].TelegramChatID, AccessHash: candidates[0].TelegramAccessHash},
			ID:      ids,
		})
	} else {
		result, err = api.MessagesGetMessages(ctx, ids)
	}
	if err != nil {
		return fmt.Errorf("fetch stored telegram messages: %w", err)
	}
	modified, ok := result.AsModified()
	if !ok {
		return fmt.Errorf("telegram returned messages without entities")
	}
	users := tg.UserClassArray(modified.GetUsers())
	chats := tg.ChatClassArray(modified.GetChats())
	entities := messagepeer.NewEntities(users.UserToMap(), chats.ChatToMap(), chats.ChannelToMap())
	for _, messageClass := range modified.GetMessages() {
		msg, ok := messageClass.(*tg.Message)
		if !ok {
			continue
		}
		candidate, ok := byTelegramID[msg.ID]
		if !ok {
			continue
		}
		entity, ok := senderEntityFromHistory(accountID, msg, entities)
		if !ok {
			continue
		}
		entityID, err := s.upsertEntityAssets(ctx, entity)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(msg)
		if err := s.store.Messages.AttachSenderEntity(ctx, candidate.MessageID, entityID, string(payload)); err != nil {
			return err
		}
		if asset, ok := messageMediaAsset(accountID, msg); ok {
			if err := s.store.Assets.UpsertMessage(ctx, candidate.ChatID, msg.ID, asset); err != nil {
				return err
			}
		}
	}
	return nil
}

// reconcileDialogAvatars 从当前群组列表刷新群头像，失败不影响消息监听和媒体下载。
func (s *Service) reconcileDialogAvatars(ctx context.Context, accountID int64, api *tg.Client) error {
	builder := dialogsquery.NewQueryBuilder(api).GetDialogs().BatchSize(100)
	return builder.ForEach(ctx, func(ctx context.Context, elem dialogsquery.Elem) error {
		if _, ok := dialogToChat(elem); !ok {
			return nil
		}
		entity, ok := dialogEntity(accountID, elem)
		if !ok {
			return nil
		}
		_, err := s.upsertEntityAssets(ctx, entity)
		return err
	})
}

// reconcileStoredMedia 从已有 raw_json 重建媒体下载记录，让升级前的历史消息也进入队列。
func (s *Service) reconcileStoredMedia(ctx context.Context, accountID int64) error {
	const batchSize = 500
	lastID := int64(0)
	for {
		messages, err := s.store.Messages.ListMediaAfter(ctx, accountID, lastID, batchSize)
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			return nil
		}
		for _, message := range messages {
			lastID = message.ID
			asset, ok, err := storedMediaAsset(accountID, message.RawJSON)
			if err != nil {
				log.Printf("skip invalid stored telegram media: message_id=%d error=%v", message.ID, err)
				continue
			}
			if !ok {
				continue
			}
			if err := s.store.Assets.UpsertMessage(ctx, message.ChatID, message.TelegramMessageID, asset); err != nil {
				return err
			}
		}
	}
}

var errMediaTooLarge = errors.New("媒体文件超过 100 MB 自动下载上限")
var errMediaUnavailable = errors.New("Telegram 原消息已无法提供媒体")

// waitMediaPoll 使用可取消定时器等待下一轮队列查询。
func waitMediaPoll(ctx context.Context) error {
	timer := time.NewTimer(mediaPollInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// downloadAsset 下载资源；文件引用过期时刷新原消息并立即重试一次。
func (s *Service) downloadAsset(ctx context.Context, accountID int64, api *tg.Client, d *downloader.Downloader, asset model.MediaAsset) error {
	err := s.downloadAssetOnce(ctx, api, d, asset)
	if err == nil || asset.OwnerType != "message" || !tgerr.Is(err, "FILE_REFERENCE_EXPIRED") {
		return err
	}
	refreshed, refreshErr := s.refreshExpiredMedia(ctx, accountID, api, asset)
	if refreshErr != nil {
		return fmt.Errorf("refresh expired media asset %d: %w", asset.ID, refreshErr)
	}
	return s.downloadAssetOnce(ctx, api, d, refreshed)
}

// refreshExpiredMedia 重新获取资源所属消息并持久化最新 Telegram 文件引用。
func (s *Service) refreshExpiredMedia(ctx context.Context, accountID int64, api *tg.Client, asset model.MediaAsset) (model.MediaAsset, error) {
	candidate, err := s.store.Messages.GetMediaMessageCandidate(ctx, accountID, asset.MessageID)
	if err != nil {
		return model.MediaAsset{}, err
	}
	message, err := fetchMediaMessageWithWait(ctx, api, candidate)
	if err != nil {
		return model.MediaAsset{}, err
	}
	refreshed, ok := messageMediaAsset(accountID, message)
	if !ok {
		return model.MediaAsset{}, fmt.Errorf("%w: refreshed message contains no downloadable media", errMediaUnavailable)
	}
	if err := s.store.Assets.RefreshMessageLocation(ctx, asset.ID, refreshed); err != nil {
		return model.MediaAsset{}, err
	}
	refreshed.ID = asset.ID
	refreshed.MessageID = asset.MessageID
	refreshed.OwnerType = asset.OwnerType
	refreshed.ForceDownload = asset.ForceDownload
	return refreshed, nil
}

// fetchMediaMessageWithWait 获取单条原消息，遇到 FLOOD_WAIT 时按 Telegram 要求等待。
func fetchMediaMessageWithWait(ctx context.Context, api *tg.Client, candidate store.MessageEntityCandidate) (*tg.Message, error) {
	for {
		message, err := fetchMediaMessage(ctx, api, candidate)
		if err == nil {
			return message, nil
		}
		var floodErr *FloodWaitError
		if !errors.As(wrapTelegramError(err), &floodErr) {
			return nil, err
		}
		log.Printf("telegram media reference refresh waiting: chat_id=%d wait=%s", candidate.ChatID, floodErr.Wait)
		timer := time.NewTimer(floodErr.Wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

// fetchMediaMessage 根据群组类型调用对应接口并返回指定 Telegram 消息。
func fetchMediaMessage(ctx context.Context, api *tg.Client, candidate store.MessageEntityCandidate) (*tg.Message, error) {
	id := []tg.InputMessageClass{&tg.InputMessageID{ID: candidate.TelegramMessageID}}
	var result tg.MessagesMessagesClass
	var err error
	if candidate.ChatType == "supergroup" {
		result, err = api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: candidate.TelegramChatID, AccessHash: candidate.TelegramAccessHash},
			ID:      id,
		})
	} else {
		result, err = api.MessagesGetMessages(ctx, id)
	}
	if err != nil {
		return nil, fmt.Errorf("fetch telegram media message %d: %w", candidate.TelegramMessageID, err)
	}
	modified, ok := result.AsModified()
	if !ok {
		return nil, fmt.Errorf("%w: unexpected messages response %T", errMediaUnavailable, result)
	}
	for _, messageClass := range modified.GetMessages() {
		if message, ok := messageClass.(*tg.Message); ok && message.ID == candidate.TelegramMessageID {
			return message, nil
		}
	}
	return nil, fmt.Errorf("%w: message %d was not returned", errMediaUnavailable, candidate.TelegramMessageID)
}

// downloadAssetOnce 将资源写入同目录临时文件，校验大小后再原子替换正式文件。
func (s *Service) downloadAssetOnce(ctx context.Context, api *tg.Client, d *downloader.Downloader, asset model.MediaAsset) error {
	if asset.FileSize > mediaDownloadLimit && !asset.ForceDownload {
		return errMediaTooLarge
	}
	location, err := assetLocation(asset)
	if err != nil {
		return err
	}
	relativePath := assetRelativePath(asset)
	finalPath := filepath.Join(s.mediaDir, relativePath)
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return fmt.Errorf("create media directory for asset %d: %w", asset.ID, err)
	}
	temporaryPath := finalPath + ".part"
	if _, err := d.Download(api, location).WithThreads(4).ToPath(ctx, temporaryPath); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("download telegram asset %d: %w", asset.ID, err)
	}
	info, err := os.Stat(temporaryPath)
	if err != nil {
		return fmt.Errorf("stat downloaded asset %d: %w", asset.ID, err)
	}
	if info.Size() > mediaDownloadLimit && !asset.ForceDownload {
		_ = os.Remove(temporaryPath)
		return errMediaTooLarge
	}
	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		return fmt.Errorf("secure downloaded asset %d: %w", asset.ID, err)
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return fmt.Errorf("publish downloaded asset %d: %w", asset.ID, err)
	}
	if err := s.store.Assets.Complete(ctx, asset.ID, relativePath, info.Size()); err != nil {
		return err
	}
	return nil
}

// assetLocation 根据持久字段重建 gotd 下载位置。
func assetLocation(asset model.MediaAsset) (tg.InputFileLocationClass, error) {
	switch asset.LocationType {
	case "photo":
		return &tg.InputPhotoFileLocation{
			ID: asset.TelegramFileID, AccessHash: asset.TelegramAccessHash,
			FileReference: asset.FileReference, ThumbSize: asset.ThumbSize,
		}, nil
	case "document":
		return &tg.InputDocumentFileLocation{
			ID: asset.TelegramFileID, AccessHash: asset.TelegramAccessHash,
			FileReference: asset.FileReference,
		}, nil
	case "peer_photo":
		peer, err := assetInputPeer(asset)
		if err != nil {
			return nil, err
		}
		location := &tg.InputPeerPhotoFileLocation{Peer: peer, PhotoID: asset.PhotoID}
		location.SetBig(true)
		return location, nil
	default:
		return nil, fmt.Errorf("unsupported media location type %q", asset.LocationType)
	}
}

// assetInputPeer 为头像下载构造账号可访问的 Telegram peer。
func assetInputPeer(asset model.MediaAsset) (tg.InputPeerClass, error) {
	switch asset.PeerType {
	case "user":
		return &tg.InputPeerUser{UserID: asset.PeerID, AccessHash: asset.PeerAccessHash}, nil
	case "channel":
		return &tg.InputPeerChannel{ChannelID: asset.PeerID, AccessHash: asset.PeerAccessHash}, nil
	case "chat":
		return &tg.InputPeerChat{ChatID: asset.PeerID}, nil
	default:
		return nil, fmt.Errorf("unsupported avatar peer type %q", asset.PeerType)
	}
}

// assetRelativePath 生成不包含用户输入目录的稳定相对路径。
func assetRelativePath(asset model.MediaAsset) string {
	owner := "messages"
	ownerID := asset.MessageID
	if asset.OwnerType == "avatar" {
		owner = "avatars"
		ownerID = asset.EntityID
	}
	return filepath.Join(owner, strconv.FormatInt(ownerID, 10), strconv.FormatInt(asset.ID, 10)+"-"+sanitizeFileName(asset.FileName))
}

// mediaRetryDelay 按一分钟起步、三倍递增计算应用层退避。
func mediaRetryDelay(retryCount int) time.Duration {
	delay := time.Minute
	for index := 0; index < retryCount; index++ {
		delay *= 3
	}
	return delay
}
