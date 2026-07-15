import { notFound } from "next/navigation";
import { ChatMessagesPanel } from "@/components/chat-messages-panel";

type ChatMessagesPageProps = {
  params: Promise<{ chatId: string }>;
};

export default async function ChatMessagesPage(props: ChatMessagesPageProps) {
  const params = await props.params;
  if (!/^\d+$/.test(params.chatId) || Number(params.chatId) < 1) {
    notFound();
  }

  return <ChatMessagesPanel chatId={Number(params.chatId)} />;
}
