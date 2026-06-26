import React from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Badge } from "@/components/ui/badge.tsx";
import { Button } from "@/components/ui/button.tsx";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog.tsx";
import { Copy, Download, Eye, Link, Package, RefreshCw, RotateCcw, Trash2, X } from "lucide-react";
import type { PhotoTask } from "@/api/photo";
import { getDownloadFileUrl, getDownloadZipUrl } from "@/api/photo";
import { useClipboard } from "@/utils/dom.ts";
import { openWindow } from "@/utils/device.ts";

interface Props {
  tasks: PhotoTask[];
  onDelete: (taskId: string) => void;
  onRetry: (taskId: string) => void;
  onRefreshTask: (taskId: string) => void;
  onRefreshAll: () => void;
}

const STATUS_COLOR: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  pending: "secondary",
  processing: "default",
  success: "default",
  failed: "destructive",
};

// 把后端/上游的英文报错映射成用户看得懂的本地化提示
function friendlyError(raw: string, t: TFunction): string {
  if (!raw) return "";
  const lower = raw.toLowerCase();
  if (lower.includes("video not supported") || lower.includes("不是视频模型"))
    return t("photo.errors.video-not-supported");
  if (lower.includes("channels are exhausted") || lower.includes("unknown channel type"))
    return t("photo.errors.no-channel");
  if (lower.includes("timeout") || lower.includes("超时"))
    return t("photo.errors.timeout");
  if (lower.includes("at least") || lower.includes("需要至少") || lower.includes("参考图"))
    return t("photo.errors.need-image");
  if (lower.includes("quota") || lower.includes("insufficient") || lower.includes("余额") || lower.includes("积分"))
    return t("photo.errors.insufficient");
  return raw;
}

const isVideoUrl = (url: string) => url.endsWith(".mp4") || url.endsWith(".webm");

// 灯箱：包裹任意 trigger，点击后弹出大图/视频预览，支持复制链接、新窗口打开、下载
const ResultLightbox: React.FC<{ url: string; index: number; trigger: React.ReactNode }> = ({ url, index, trigger }) => {
  const { t } = useTranslation();
  const copy = useClipboard();
  const video = isVideoUrl(url);
  const label = video ? t("photo.task.video", { n: index + 1 }) : t("photo.task.result", { n: index + 1 });

  return (
    <Dialog>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle className="flex items-center text-base">
            <Eye className="h-4 w-4 mr-1.5" /> {label}
          </DialogTitle>
        </DialogHeader>
        <div className="flex justify-end gap-2 mb-2">
          <Button size="icon" variant="outline" onClick={() => copy(url)} title={t("photo.task.copy-link")}>
            <Copy className="h-4 w-4" />
          </Button>
          <Button size="icon" variant="outline" onClick={() => openWindow(url)} title={t("photo.task.open-window")}>
            <Link className="h-4 w-4" />
          </Button>
          <a href={getDownloadFileUrl(url)} download>
            <Button size="icon" variant="outline" title={t("photo.task.download")}>
              <Download className="h-4 w-4" />
            </Button>
          </a>
        </div>
        <div className="flex justify-center max-h-[70vh] overflow-auto">
          {video ? (
            <video src={url} controls className="max-w-full rounded-md" />
          ) : (
            <img src={url} alt={label} className="max-w-full rounded-md object-contain" />
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
};

// 结果项（展开区）：卡片缩略图 + 文件名 + 下载，点击进灯箱
const ResultPreview: React.FC<{ url: string; index: number }> = ({ url, index }) => {
  const { t } = useTranslation();
  const video = isVideoUrl(url);
  const label = video ? t("photo.task.video", { n: index + 1 }) : t("photo.task.result", { n: index + 1 });

  return (
    <div className="border rounded-md overflow-hidden w-40 bg-card">
      <ResultLightbox
        url={url}
        index={index}
        trigger={
          <div className="cursor-pointer group relative">
            {video ? (
              <video src={url} className="w-full h-28 object-cover bg-muted" />
            ) : (
              <img src={url} alt={label} className="w-full h-28 object-contain bg-muted" />
            )}
            <div className="absolute inset-0 flex items-center justify-center bg-black/0 group-hover:bg-black/30 transition-colors">
              <Eye className="h-5 w-5 text-white opacity-0 group-hover:opacity-100 transition-opacity" />
            </div>
          </div>
        }
      />
      <div className="flex justify-between items-center p-1">
        <span className="text-xs text-muted-foreground">{label}</span>
        <a href={getDownloadFileUrl(url)} download className="text-primary" title={t("photo.task.download")}>
          <Download className="h-3 w-3" />
        </a>
      </div>
    </div>
  );
};

// 行内缩略图：成功任务无需展开即可看到结果；悬停浮层放大，点击进灯箱
const InlineThumb: React.FC<{ url: string; index: number }> = ({ url, index }) => {
  const video = isVideoUrl(url);
  return (
    <ResultLightbox
      url={url}
      index={index}
      trigger={
        <button
          type="button"
          className="relative group/thumb h-10 w-10 shrink-0"
          onClick={(e) => e.stopPropagation()}
        >
          <span className="block h-10 w-10 overflow-hidden rounded border bg-muted">
            {video ? (
              <video src={url} className="h-full w-full object-cover" />
            ) : (
              <img src={url} alt="" className="h-full w-full object-cover" />
            )}
          </span>
          {/* 悬停浮层放大预览 */}
          <span className="pointer-events-none absolute bottom-full left-1/2 z-50 mb-1 hidden -translate-x-1/2 group-hover/thumb:block">
            <span className="block rounded-md border bg-popover p-1 shadow-lg">
              {video ? (
                <video src={url} className="h-36 w-36 object-contain" />
              ) : (
                <img src={url} alt="" className="h-36 w-36 object-contain" />
              )}
            </span>
          </span>
        </button>
      }
    />
  );
};

const TaskRow: React.FC<{
  task: PhotoTask;
  onDelete: (id: string) => void;
  onRetry: (id: string) => void;
  onRefresh: (id: string) => void;
}> = ({ task, onDelete, onRetry, onRefresh }) => {
  const { t } = useTranslation();
  const [expanded, setExpanded] = React.useState(false);
  const stColor = STATUS_COLOR[task.status] || "secondary";
  const stLabel = t(`photo.status.${task.status}`, task.status);
  const isActive = ["pending", "processing"].includes(task.status);
  const results = task.result_urls ?? [];

  return (
    <div className="border rounded-md mb-2 bg-card">
      <div className="flex items-center p-3 gap-3 cursor-pointer" onClick={() => setExpanded(!expanded)}>
        <span className="font-mono text-xs text-muted-foreground w-20 truncate">{task.task_id}</span>
        <span className="text-sm w-16">{t(`photo.features.${task.feature}`, task.feature)}</span>
        <Badge variant={stColor}>{stLabel}</Badge>
        <div className="flex-1 mx-2">
          <div className="h-2 bg-muted rounded-full overflow-hidden">
            <div className={`h-full rounded-full transition-all ${task.status === "failed" ? "bg-destructive" : "bg-primary"}`}
              style={{ width: `${task.progress}%` }} />
          </div>
        </div>
        <span className="text-xs text-muted-foreground">
          {task.processed_images}/{task.total_images}
          {task.total_videos > 0 && ` +${task.processed_videos}V`}
        </span>
        <span className="text-xs text-muted-foreground hidden sm:inline">{task.created_at?.slice(0, 16)}</span>

        {/* 行内结果缩略图：成功任务无需展开即可预览（窄屏隐藏，避免溢出） */}
        {task.status === "success" && results.length > 0 && (
          <div className="hidden md:flex items-center gap-1">
            {results.slice(0, 3).map((url, i) => (
              <InlineThumb key={i} url={url} index={i} />
            ))}
            {results.length > 3 && (
              <span className="text-xs text-muted-foreground">+{results.length - 3}</span>
            )}
          </div>
        )}

        <div className="flex gap-1" onClick={(e) => e.stopPropagation()}>
          {isActive && (
            <>
              <Button size="sm" variant="ghost" onClick={() => onRefresh(task.task_id)} title={t("photo.task.refresh")}>
                <RefreshCw className="h-3 w-3" />
              </Button>
              <Button size="sm" variant="ghost" className="text-destructive" onClick={() => onDelete(task.task_id)} title={t("photo.task.cancel-task")}>
                <X className="h-3 w-3 mr-1" />{t("photo.task.cancel")}
              </Button>
            </>
          )}
          {task.status === "failed" && (
            <Button size="sm" variant="default" onClick={() => onRetry(task.task_id)}>
              <RotateCcw className="h-3 w-3 mr-1" />{t("photo.task.retry")}
            </Button>
          )}
          {!isActive && (
            <Button size="sm" variant="ghost" className="text-destructive" onClick={() => onDelete(task.task_id)} title={t("photo.task.delete")}>
              <Trash2 className="h-3 w-3" />
            </Button>
          )}
        </div>
      </div>

      {/* Expanded row */}
      {expanded && (
        <div className="px-4 pb-3 border-t pt-2">
          {task.error_message && <p className="text-destructive text-sm mb-2">{t("photo.task.error", { msg: friendlyError(task.error_message, t) })}</p>}
          {(task.source_filenames?.length ?? 0) > 0 && (
            <p className="text-muted-foreground text-xs mb-2">{t("photo.task.source-files", { files: task.source_filenames.join(", ") })}</p>
          )}
          {results.length > 0 && (
            <>
              {results.length > 1 && (
                <div className="flex justify-end mb-2">
                  <a href={getDownloadZipUrl(results)} download>
                    <Button size="sm" variant="outline">
                      <Package className="h-3 w-3 mr-1" />{t("photo.task.zip-download", { count: results.length })}
                    </Button>
                  </a>
                </div>
              )}
              <div className="flex flex-wrap gap-2">
                {results.map((url, i) => (
                  <ResultPreview key={i} url={url} index={i} />
                ))}
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
};

const TaskTable: React.FC<Props> = ({ tasks, onDelete, onRetry, onRefreshTask, onRefreshAll }) => {
  const { t } = useTranslation();
  const [tab, setTab] = React.useState<"active" | "history">("active");

  const activeTasks = tasks.filter((tk) => ["pending", "processing"].includes(tk.status));
  const historyTasks = tasks.filter((tk) => ["success", "failed"].includes(tk.status));

  const list = tab === "active" ? activeTasks : historyTasks;

  return (
    <div className="p-4">
      <div className="flex items-center justify-between mb-3">
        <div className="flex gap-2">
          <Button size="sm" variant={tab === "active" ? "default" : "outline"}
            onClick={() => setTab("active")}>
            {t("photo.task.active", { count: activeTasks.length })}
          </Button>
          <Button size="sm" variant={tab === "history" ? "default" : "outline"}
            onClick={() => setTab("history")}>
            {t("photo.task.history", { count: historyTasks.length })}
          </Button>
        </div>
        <Button size="sm" variant="ghost" onClick={onRefreshAll}>
          <RefreshCw className="h-3 w-3 mr-1" />{t("photo.task.refresh")}
        </Button>
      </div>

      {list.length === 0 ? (
        <p className="text-center text-muted-foreground py-8">{tab === "active" ? t("photo.task.empty-active") : t("photo.task.empty-history")}</p>
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
