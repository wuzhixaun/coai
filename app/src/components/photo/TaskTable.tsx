import React from "react";
import { Badge } from "@/components/ui/badge.tsx";
import { Button } from "@/components/ui/button.tsx";
import { Download, RefreshCw, RotateCcw, Trash2 } from "lucide-react";
import type { PhotoTask } from "@/api/photo";
import { getDownloadFileUrl } from "@/api/photo";

interface Props {
  tasks: PhotoTask[];
  onDelete: (taskId: string) => void;
  onRetry: (taskId: string) => void;
  onRefreshTask: (taskId: string) => void;
  onRefreshAll: () => void;
}

const STATUS_MAP: Record<string, { label: string; color: "default" | "secondary" | "destructive" | "outline" }> = {
  pending: { label: "排队中", color: "secondary" },
  processing: { label: "处理中", color: "default" },
  success: { label: "已完成", color: "default" },
  failed: { label: "失败", color: "destructive" },
};

const FEATURE_LABEL: Record<string, string> = {
  white_bg: "白底图", scene_gen: "场景图", image_erase: "擦除", color_change: "换色",
  marketing: "营销图", image_translate: "翻译", hd_upscale: "高清", model_image: "模特图",
  material_change: "换材质", instruction_gen: "指令生图", detail_image: "细节图",
  logo_custom: "Logo定制", production_flow: "流程图", resize: "改尺寸", video_gen: "视频",
};

const TaskRow: React.FC<{ task: PhotoTask; onDelete: (id: string) => void; onRetry: (id: string) => void; onRefresh: (id: string) => void }> =
  ({ task, onDelete, onRetry, onRefresh }) => {
  const [expanded, setExpanded] = React.useState(false);
  const st = STATUS_MAP[task.status] || { label: task.status, color: "secondary" as const };

  return (
    <div className="border rounded mb-2 bg-white">
      <div className="flex items-center p-3 gap-3 cursor-pointer" onClick={() => setExpanded(!expanded)}>
        <span className="font-mono text-xs text-gray-500 w-20 truncate">{task.task_id}</span>
        <span className="text-sm w-16">{FEATURE_LABEL[task.feature] || task.feature}</span>
        <Badge variant={st.color}>{st.label}</Badge>
        <div className="flex-1 mx-2">
          <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
            <div className={`h-full rounded-full transition-all ${task.status === "failed" ? "bg-red-500" : "bg-primary"}`}
              style={{ width: `${task.progress}%` }} />
          </div>
        </div>
        <span className="text-xs text-gray-500">
          {task.processed_images}/{task.total_images}
          {task.total_videos > 0 && ` +${task.processed_videos}V`}
        </span>
        <span className="text-xs text-gray-400">{task.created_at?.slice(0, 16)}</span>

        <div className="flex gap-1" onClick={(e) => e.stopPropagation()}>
          {["pending", "processing"].includes(task.status) && (
            <Button size="sm" variant="ghost" onClick={() => onRefresh(task.task_id)}>
              <RefreshCw className="h-3 w-3" />
            </Button>
          )}
          {task.status === "failed" && (
            <Button size="sm" variant="default" onClick={() => onRetry(task.task_id)}>
              <RotateCcw className="h-3 w-3 mr-1" />重试
            </Button>
          )}
          <Button size="sm" variant="ghost" className="text-red-500" onClick={() => onDelete(task.task_id)}>
            <Trash2 className="h-3 w-3" />
          </Button>
        </div>
      </div>

      {/* Expanded row */}
      {expanded && (
        <div className="px-4 pb-3 border-t pt-2">
          {task.error_message && <p className="text-red-500 text-sm mb-2">错误: {task.error_message}</p>}
          {(task.source_filenames?.length ?? 0) > 0 && (
            <p className="text-gray-500 text-xs mb-2">源文件: {task.source_filenames.join(", ")}</p>
          )}
          {(task.result_urls?.length ?? 0) > 0 && (
            <div className="flex flex-wrap gap-2">
              {task.result_urls.map((url, i) => {
                const isVideo = url.endsWith(".mp4") || url.endsWith(".webm");
                return (
                  <div key={i} className="border rounded overflow-hidden w-40">
                    {isVideo ? (
                      <video src={url} controls className="w-full h-24 object-cover" />
                    ) : (
                      <img src={url} alt={`结果${i + 1}`} className="w-full h-24 object-contain bg-gray-100" />
                    )}
                    <div className="flex justify-between items-center p-1">
                      <span className="text-xs">{isVideo ? "视频" : "结果"} {i + 1}</span>
                      <a href={getDownloadFileUrl(url)} download className="text-primary">
                        <Download className="h-3 w-3" />
                      </a>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
};

const TaskTable: React.FC<Props> = ({ tasks, onDelete, onRetry, onRefreshTask, onRefreshAll }) => {
  const [tab, setTab] = React.useState<"active" | "history">("active");

  const activeTasks = tasks.filter((t) => ["pending", "processing"].includes(t.status));
  const historyTasks = tasks.filter((t) => ["success", "failed"].includes(t.status));

  const list = tab === "active" ? activeTasks : historyTasks;

  return (
    <div className="p-4">
      <div className="flex items-center justify-between mb-3">
        <div className="flex gap-2">
          <Button size="sm" variant={tab === "active" ? "default" : "outline"}
            onClick={() => setTab("active")}>
            当前处理 ({activeTasks.length})
          </Button>
          <Button size="sm" variant={tab === "history" ? "default" : "outline"}
            onClick={() => setTab("history")}>
            历史记录 ({historyTasks.length})
          </Button>
        </div>
        <Button size="sm" variant="ghost" onClick={onRefreshAll}>
          <RefreshCw className="h-3 w-3 mr-1" />刷新
        </Button>
      </div>

      {list.length === 0 ? (
        <p className="text-center text-gray-400 py-8">暂无{tab === "active" ? "进行中" : "历史"}任务</p>
      ) : (
        list.map((task) => (
          <TaskRow key={task.task_id} task={task}
            onDelete={onDelete} onRetry={onRetry} onRefresh={onRefreshTask} />
        ))
      )}
    </div>
  );
};

export default TaskTable;
