import { Button } from "./ui/button.tsx";
import { useConversationActions, useMessages } from "@/store/chat.ts";
import { MessageSquarePlus } from "lucide-react";

function ProjectLink() {
  const messages = useMessages();
  const { toggle } = useConversationActions();

  // 有消息时显示「新建对话」按钮；无消息时不再显示 GitHub 项目入口图标。
  return messages.length > 0 ? (
    <Button
      variant="outline"
      size="icon-md"
      className="rounded-full overflow-hidden"
      onClick={async () => await toggle(-1)}
    >
      <MessageSquarePlus className={`h-4 w-4`} />
    </Button>
  ) : null;
}

export default ProjectLink;
