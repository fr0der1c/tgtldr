package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TelegramEntityRepository struct {
	pool *pgxpool.Pool
}

// Upsert 保存下载头像所需的 Telegram 实体引用，并返回本地实体 ID。
func (r *TelegramEntityRepository) Upsert(ctx context.Context, entity model.TelegramEntity) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		insert into telegram_entities (
			telegram_account_id, peer_type, telegram_id, access_hash,
			display_name, username, current_photo_id
		) values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (telegram_account_id, peer_type, telegram_id) do update
		set access_hash = excluded.access_hash,
		    display_name = excluded.display_name,
		    username = excluded.username,
		    current_photo_id = excluded.current_photo_id,
		    updated_at = now()
		returning id
	`, entity.TelegramAccountID, entity.PeerType, entity.TelegramID, entity.AccessHash,
		entity.DisplayName, entity.Username, entity.CurrentPhotoID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert telegram entity %s %d: %w", entity.PeerType, entity.TelegramID, err)
	}
	return id, nil
}

type MediaAssetRepository struct {
	pool *pgxpool.Pool
}

// UpsertMessage 为已入库消息创建或刷新媒体下载记录。
func (r *MediaAssetRepository) UpsertMessage(ctx context.Context, chatID int64, telegramMessageID int, asset model.MediaAsset) error {
	_, err := r.pool.Exec(ctx, `
		insert into media_assets (
			telegram_account_id, owner_type, message_id, kind, mime_type, file_name,
			file_size, location_type, telegram_file_id, telegram_access_hash,
			file_reference, thumb_size, status
		)
		select $3, 'message', m.id, $4, $5, $6, $7::bigint, $8, $9, $10, $11, $12,
		       case when $7::bigint > 104857600 then 'skipped_oversize' else 'pending' end
		from messages m where m.chat_id = $1 and m.telegram_message_id = $2
		on conflict (message_id) where owner_type = 'message' do update
		set telegram_account_id = excluded.telegram_account_id,
		    kind = excluded.kind, mime_type = excluded.mime_type, file_name = excluded.file_name,
		    file_size = excluded.file_size, location_type = excluded.location_type,
		    telegram_file_id = excluded.telegram_file_id,
		    telegram_access_hash = excluded.telegram_access_hash,
		    file_reference = excluded.file_reference, thumb_size = excluded.thumb_size,
		    status = case
		      when media_assets.status in ('pending', 'downloading', 'succeeded', 'failed') then media_assets.status
		      when excluded.file_size > 104857600 and not media_assets.force_download then 'skipped_oversize'
		      else 'pending'
		    end,
		    next_retry_at = case when media_assets.status = 'pending' then media_assets.next_retry_at else null end,
		    error_message = case
		      when media_assets.status in ('pending', 'failed') then media_assets.error_message
		      else ''
		    end,
		    updated_at = now()
	`, chatID, telegramMessageID, asset.TelegramAccountID, asset.Kind, asset.MIMEType,
		asset.FileName, asset.FileSize, asset.LocationType, asset.TelegramFileID,
		asset.TelegramAccessHash, asset.FileReference, asset.ThumbSize)
	if err != nil {
		return fmt.Errorf("upsert message media %d: %w", telegramMessageID, err)
	}
	return nil
}

// RefreshMessageLocation 更新过期的 Telegram 文件引用，不改变当前下载任务状态。
func (r *MediaAssetRepository) RefreshMessageLocation(ctx context.Context, id int64, asset model.MediaAsset) error {
	result, err := r.pool.Exec(ctx, `
		update media_assets
		set kind = $2, mime_type = $3, file_name = $4, file_size = $5,
		    location_type = $6, telegram_file_id = $7, telegram_access_hash = $8,
		    file_reference = $9, thumb_size = $10, updated_at = now()
		where id = $1 and owner_type = 'message'
	`, id, asset.Kind, asset.MIMEType, asset.FileName, asset.FileSize,
		asset.LocationType, asset.TelegramFileID, asset.TelegramAccessHash,
		asset.FileReference, asset.ThumbSize)
	if err != nil {
		return fmt.Errorf("refresh message media asset %d: %w", id, err)
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// UpsertAvatar 为实体的当前头像创建下载记录，已存在时保持原下载状态。
func (r *MediaAssetRepository) UpsertAvatar(ctx context.Context, entity model.TelegramEntity, entityID int64) error {
	if entity.CurrentPhotoID == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		insert into media_assets (
			telegram_account_id, owner_type, entity_id, photo_id, kind, mime_type,
			file_name, location_type, peer_type, peer_id, peer_access_hash, status
		) values ($1, 'avatar', $2, $3, 'avatar', 'image/jpeg', $4, 'peer_photo', $5, $6, $7, 'pending')
		on conflict (entity_id, photo_id) where owner_type = 'avatar' do nothing
	`, entity.TelegramAccountID, entityID, entity.CurrentPhotoID,
		fmt.Sprintf("avatar-%d.jpg", entity.CurrentPhotoID), entity.PeerType,
		entity.TelegramID, entity.AccessHash)
	if err != nil {
		return fmt.Errorf("upsert avatar %s %d: %w", entity.PeerType, entity.TelegramID, err)
	}
	return nil
}

// RecoverInterrupted 将进程退出时遗留的下载中任务重新排队。
func (r *MediaAssetRepository) RecoverInterrupted(ctx context.Context, accountID int64) error {
	_, err := r.pool.Exec(ctx, `
		update media_assets set status = 'pending', updated_at = now()
		where telegram_account_id = $1 and status = 'downloading'
	`, accountID)
	if err != nil {
		return fmt.Errorf("recover interrupted media downloads: %w", err)
	}
	return nil
}

// ClaimNext 原子领取一个到期任务，保证同一资源不会被重复下载。
func (r *MediaAssetRepository) ClaimNext(ctx context.Context, accountID int64) (*model.MediaAsset, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin claim media asset: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, mediaAssetSelect+`
		where telegram_account_id = $1
		  and status = 'pending'
		  and (next_retry_at is null or next_retry_at <= now())
		order by created_at desc
		for update skip locked limit 1
	`, accountID)
	asset, err := scanMediaAsset(row)
	if errorsIsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `update media_assets set status = 'downloading', updated_at = now() where id = $1`, asset.ID); err != nil {
		return nil, fmt.Errorf("mark media asset downloading: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claimed media asset: %w", err)
	}
	asset.Status = "downloading"
	return &asset, nil
}

// errorsIsNoRows 保留 pgx 错误链判断，供领取空队列使用。
func errorsIsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// Complete 记录原子落盘后的资源路径和真实大小。
func (r *MediaAssetRepository) Complete(ctx context.Context, id int64, localPath string, size int64) error {
	_, err := r.pool.Exec(ctx, `
		update media_assets set status = 'succeeded', local_path = $2, file_size = $3,
		       error_message = '', next_retry_at = null, downloaded_at = now(), updated_at = now()
		where id = $1
	`, id, localPath, size)
	if err != nil {
		return fmt.Errorf("complete media asset %d: %w", id, err)
	}
	return nil
}

// Fail 根据错误是否可重试，安排下次执行或沉淀最终失败状态。
func (r *MediaAssetRepository) Fail(ctx context.Context, id int64, message string, retry bool, delay time.Duration) error {
	status := "failed"
	var next *time.Time
	if retry {
		status = "pending"
		value := time.Now().Add(delay)
		next = &value
	}
	_, err := r.pool.Exec(ctx, `
		update media_assets set status = $2, retry_count = retry_count + 1,
		       next_retry_at = $3, error_message = $4, updated_at = now()
		where id = $1
	`, id, status, next, message)
	if err != nil {
		return fmt.Errorf("fail media asset %d: %w", id, err)
	}
	return nil
}

// RequestDownload 允许用户重试失败资源或显式下载超限资源。
func (r *MediaAssetRepository) RequestDownload(ctx context.Context, id int64) error {
	result, err := r.pool.Exec(ctx, `
		update media_assets set status = 'pending', force_download = true,
		       retry_count = 0, next_retry_at = null, error_message = '', updated_at = now()
		where id = $1 and status in ('failed', 'skipped_oversize')
	`, id)
	if err != nil {
		return fmt.Errorf("request media download %d: %w", id, err)
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *MediaAssetRepository) Get(ctx context.Context, id int64) (model.MediaAsset, error) {
	asset, err := scanMediaAsset(r.pool.QueryRow(ctx, mediaAssetSelect+` where id = $1`, id))
	if err != nil {
		return model.MediaAsset{}, fmt.Errorf("get media asset %d: %w", id, err)
	}
	return asset, nil
}

// FindEntityAvatar 返回实体最近一次成功下载的头像，便于新头像下载时继续显示旧图。
func (r *MediaAssetRepository) FindEntityAvatar(ctx context.Context, accountID int64, peerType string, telegramID int64) (model.MediaAsset, error) {
	asset, err := scanMediaAsset(r.pool.QueryRow(ctx, mediaAssetSelect+`
		join telegram_entities e on e.id = a.entity_id
		where e.telegram_account_id = $1 and e.peer_type = $2 and e.telegram_id = $3
		  and a.owner_type = 'avatar' and a.status = 'succeeded'
		order by a.downloaded_at desc limit 1
	`, accountID, peerType, telegramID))
	if err != nil {
		return model.MediaAsset{}, fmt.Errorf("find entity avatar %s %d: %w", peerType, telegramID, err)
	}
	return asset, nil
}

// ListForMessages 批量返回消息媒体和当前可用的发言人头像。
func (r *MediaAssetRepository) ListForMessages(ctx context.Context, messageIDs []int64) (map[int64]model.MediaAsset, map[int64]model.MediaAsset, error) {
	media := make(map[int64]model.MediaAsset)
	avatars := make(map[int64]model.MediaAsset)
	if len(messageIDs) == 0 {
		return media, avatars, nil
	}
	rows, err := r.pool.Query(ctx, `
		select `+mediaAssetColumns+`, m.id as source_message_id, a.owner_type
		from messages m
		join media_assets a on
		  (a.owner_type = 'message' and a.message_id = m.id) or
		  (a.owner_type = 'avatar' and a.entity_id = m.sender_entity_id and a.status = 'succeeded')
		where m.id = any($1)
		order by a.downloaded_at desc nulls last
	`, messageIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("list message media assets: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		asset, err := scanMediaAssetWithMessage(rows)
		if err != nil {
			return nil, nil, err
		}
		messageID := asset.MessageID
		if asset.OwnerType == "avatar" {
			if _, exists := avatars[messageID]; !exists {
				avatars[messageID] = asset
			}
			continue
		}
		media[messageID] = asset
	}
	return media, avatars, rows.Err()
}

const mediaAssetColumns = `
	a.id, coalesce(a.telegram_account_id, 0), a.owner_type,
	coalesce(a.message_id, 0), coalesce(a.entity_id, 0), a.photo_id,
	a.kind, a.mime_type, a.file_name, a.file_size, a.location_type,
	a.telegram_file_id, a.telegram_access_hash, a.file_reference, a.thumb_size,
	a.peer_type, a.peer_id, a.peer_access_hash, a.status, a.force_download,
	a.local_path, a.retry_count, a.next_retry_at, a.error_message,
	a.downloaded_at, a.created_at, a.updated_at`

const mediaAssetSelect = `select ` + mediaAssetColumns + ` from media_assets a`

type mediaAssetScanner interface {
	Scan(dest ...any) error
}

// scanMediaAsset 统一维护资源表字段扫描顺序。
func scanMediaAsset(scanner mediaAssetScanner) (model.MediaAsset, error) {
	var asset model.MediaAsset
	err := scanner.Scan(
		&asset.ID, &asset.TelegramAccountID, &asset.OwnerType,
		&asset.MessageID, &asset.EntityID, &asset.PhotoID,
		&asset.Kind, &asset.MIMEType, &asset.FileName, &asset.FileSize, &asset.LocationType,
		&asset.TelegramFileID, &asset.TelegramAccessHash, &asset.FileReference, &asset.ThumbSize,
		&asset.PeerType, &asset.PeerID, &asset.PeerAccessHash, &asset.Status, &asset.ForceDownload,
		&asset.LocalPath, &asset.RetryCount, &asset.NextRetryAt, &asset.ErrorMessage,
		&asset.DownloadedAt, &asset.CreatedAt, &asset.UpdatedAt,
	)
	if err != nil {
		return model.MediaAsset{}, err
	}
	return asset, nil
}

// scanMediaAssetWithMessage 为批量消息查询附加来源消息 ID。
func scanMediaAssetWithMessage(scanner mediaAssetScanner) (model.MediaAsset, error) {
	var sourceMessageID int64
	var ownerType string
	asset, err := scanMediaAsset(extraScanner{scanner: scanner, extra: []any{&sourceMessageID, &ownerType}})
	if err != nil {
		return model.MediaAsset{}, err
	}
	asset.MessageID = sourceMessageID
	asset.OwnerType = ownerType
	return asset, nil
}

type extraScanner struct {
	scanner mediaAssetScanner
	extra   []any
}

// Scan 在基础资源字段后追加查询专用字段。
func (s extraScanner) Scan(dest ...any) error {
	return s.scanner.Scan(append(dest, s.extra...)...)
}
