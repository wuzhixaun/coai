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
import { Brush, Columns2, Copy, Download, Eye, Image as ImageIcon, Link, Package, RefreshCw, RotateCcw, Trash2, X } from "lucide-react";
import type { PhotoImage, PhotoTask } from "@/api/photo";
import { getDownloadFileUrl, getDownloadZipUrl } from "@/api/photo";
import { useClipboard } from "@/utils/dom.ts";
import { openWindow } from "@/utils/device.ts";
import BeforeAfterSlider from "./BeforeAfterSlider";

interface Props {
  tasks: PhotoTask[];
  images?: PhotoImage[];
  onDelete: (taskId: string) => void;
  onRetry: (taskId: string) => void;
  onRefreshTask: (taskId: string) => void;
  onRefreshAll: () => void;
  onInpaint?: (url: string) => void;
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
  if (lower.includes("at most") || lower.includes("最多") || lower.includes("too many"))
    return t("photo.errors.too-many-images");
  if (lower.includes("at least") || lower.includes("需要至少") || lower.includes("参考图"))
    return t("photo.errors.need-image");
  if (lower.includes("quota") || lower.includes("insufficient") || lower.includes("余额") || lower.includes("积分"))
    return t("photo.errors.insufficient");
  return raw;
}

const isVideoUrl = (url: string) => url.endsWith(".mp4") || url.endsWith(".webm");

// 后端 created_at 多为 UTC（DB CURRENT_TIMESTAMP，无时区标记）。
// 这里按 UTC 解析再转成浏览器本地时区显示，修正"少 8 小时"。
function formatLocalTime(s?: string): string {
  if (!s) return "";
  const hasTz = /[zZ]|[+-]\d{2}:?\d{2}$/.test(s);
  const iso = hasTz ? s : s.replace(" ", "T") + "Z";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return s.slice(0, 16);
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

// 灯箱：包裹任意 trigger，点击后弹出大图/视频预览，支持复制链接、新窗口打开、下载；
// 传入 sourceUrl（且非视频）时提供「对比原图 / 仅看结果」切换。
const ResultLightbox: React.FC<{ url: string; index: number; trigger: React.ReactNode; sourceUrl?: string; onInpaint?: (url: string) => void }> = ({ url, index, trigger, sourceUrl, onInpaint }) => {
  const { t } = useTranslation();
  const copy = useClipboard();
  const video = isVideoUrl(url);
  const label = video ? t("photo.task.video", { n: index + 1 }) : t("photo.task.result", { n: index + 1 });
  const canCompare = !!sourceUrl && !video;
  const [compare, setCompare] = React.useState(false);
  const [open, setOpen] = React.useState(false);

  return (
    <Dialog open={open} onOpenChange={(o) => { setOpen(o); if (!o) setCompare(false); }}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle className="flex items-center text-base">
            <Eye className="h-4 w-4 mr-1.5" /> {label}
          </DialogTitle>
        </DialogHeader>
        <div className="flex items-center gap-2 mb-2">
          {canCompare && (
            <Button size="sm" variant={compare ? "default" : "outline"} onClick={() => setCompare((v) => !v)}>
              {compare ? <ImageIcon className="h-4 w-4 mr-1" /> : <Columns2 className="h-4 w-4 mr-1" />}
              {compare ? t("photo.task.result-only") : t("photo.task.compare")}
            </Button>
          )}
          {onInpaint && !video && (
            <Button size="sm" variant="outline" onClick={() => { setOpen(false); onInpaint(url); }}>
              <Brush className="h-4 w-4 mr-1" />{t("photo.inpaint.open")}
            </Button>
          )}
          <div className="ml-auto flex gap-2">
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
        </div>
        <div className="flex justify-center max-h-[70vh] overflow-auto">
          {compare && canCompare ? (
            <BeforeAfterSlider before={sourceUrl!} after={url} className="w-full" />
          ) : video ? (
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
const ResultPreview: React.FC<{ url: string; index: number; sourceUrl?: string; onInpaint?: (url: string) => void }> = ({ url, index, sourceUrl, onInpaint }) => {
  const { t } = useTranslation();
  const video = isVideoUrl(url);
  const label = video ? t("photo.task.video", { n: index + 1 }) : t("photo.task.result", { n: index + 1 });

  return (
    <div className="border rounded-md overflow-hidden w-40 bg-card">
      <ResultLightbox
        url={url}
        index={index}
        sourceUrl={sourceUrl}
        onInpaint={onInpaint}
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
const InlineThumb: React.FC<{ url: string; index: number; sourceUrl?: string; onInpaint?: (url: string) => void }> = ({ url, index, sourceUrl, onInpaint }) => {
  const video = isVideoUrl(url);
  return (
    <ResultLightbox
      url={url}
      index={index}
      sourceUrl={sourceUrl}
      onInpaint={onInpaint}
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
  sourceUrls?: string[];
  onDelete: (id: string) => void;
  onRetry: (id: string) => void;
  onRefresh: (id: string) => void;
  onInpaint?: (url: string) => void;
}> = ({ task, sourceUrls = [], onDelete, onRetry, onRefresh, onInpaint }) => {
  const { t } = useTranslation();
  const [expanded, setExpanded] = React.useState(false);
  const [picked, setPicked] = React.useState<Set<string>>(new Set());
  const togglePick = (url: string) => setPicked((prev) => {
    const n = new Set(prev);
    n.has(url) ? n.delete(url) : n.add(url);
    return n;
  });
  const stColor = STATUS_COLOR[task.status] || "secondary";
  const stLabel = t(`photo.status.${task.status}`, task.status);
  const isActive = ["pending", "processing"].includes(task.status);
  const results = task.result_urls ?? [];
  // 对比用源图：优先按索引配对，单源/数量不匹配时回退到首张源图
  const sourceFor = (i: number) => sourceUrls[i] ?? sourceUrls[0];
  // ZIP 文件名前缀：功能 + 首个源文件名(去扩展名)，便于分平台/批量归档
  const srcStem = (task.source_filenames?.[0] || "").replace(/\.[^.]+$/, "");
  const zipPrefix = [task.feature, srcStem].filter(Boolean).join("_") || "result";
  // 部分成功：后端任一图出错即把整任务标记 failed，但已成功的结果仍写入 result_urls。
  // 期望产出数 = total_videos 优先（视频），否则 total_images。
  const expected = task.total_videos > 0 ? task.total_videos : task.total_images;
  const succeeded = results.length;
  const isPartial = task.status === "failed" && succeeded > 0;

  return (
    <div className={`border rounded-md mb-2 bg-card ${
      task.status === "failed" ? (isPartial ? "border-amber-500/50" : "border-destructive/50") : ""
    }`}>
      <div className="flex items-center p-3 gap-3 cursor-pointer" onClick={() => setExpanded(!expanded)}>
        <span className="font-mono text-xs text-muted-foreground w-20 truncate">{task.task_id}</span>
        <span className="text-sm w-16">{t(`photo.features.${task.feature}`, task.feature)}</span>
        {isPartial ? (
          <Badge variant="outline" className="border-amber-500 text-amber-600">
            {t("photo.task.partial", { done: succeeded, total: expected })}
          </Badge>
        ) : (
          <Badge variant={stColor}>{stLabel}</Badge>
        )}
        <div className="flex-1 mx-2">
          <div className="h-2 bg-muted rounded-full overflow-hidden">
            <div className={`h-full rounded-full transition-all ${task.status === "failed" ? "bg-destructive" : "bg-primary"}`}
              style={{ width: `${task.progress}%` }} />
          </div>
          {task.status === "processing" && (
            <p className="mt-1 text-[10px] text-muted-foreground truncate">
              {task.total_videos > 0
                ? t("photo.task.progress-detail-video", { done: task.processed_videos, total: task.total_videos })
                : t("photo.task.progress-detail", {
                    feature: t(`photo.features.${task.feature}`, task.feature),
                    done: task.processed_images,
                    total: task.total_images,
                  })}
            </p>
          )}
        </div>
        <span className="text-xs text-muted-foreground">
          {task.processed_images}/{task.total_images}
          {task.total_videos > 0 && ` +${task.processed_videos}V`}
        </span>
        <span className="text-xs text-muted-foreground hidden sm:inline">{formatLocalTime(task.created_at)}</span>

        {/* 行内结果缩略图：成功/部分成功无需展开即可预览（窄屏隐藏，避免溢出） */}
        {!isActive && results.length > 0 && (
          <div className="hidden md:flex items-center gap-1">
            {results.slice(0, 3).map((url, i) => (
              <InlineThumb key={i} url={url} index={i} sourceUrl={sourceFor(i)} onInpaint={onInpaint} />
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
              <RotateCcw className="h-3 w-3 mr-1" />{isPartial ? t("photo.task.retry-missing") : t("photo.task.retry")}
            </Button>
          )}
          {!isActive && (
            <Button size="sm" variant="ghost" className="text-destructive" onClick={() => onDelete(task.task_id)}
              title={task.status === "failed" ? t("photo.task.dismiss") : t("photo.task.delete")}>
              <Trash2 className="h-3 w-3" />
            </Button>
          )}
        </div>
      </div>

      {/* Expanded row */}
      {expanded && (
        <div className="px-4 pb-3 border-t pt-2">
          {task.error_message && <p className="text-destructive text-sm mb-2">{t("photo.task.error", { msg: friendlyError(task.error_message, t) })}</p>}
          {/* 逐图状态：批量任务每张源图的成功/失败（P3.2） */}
          {(task.item_status?.length ?? 0) > 1 && (
            <div className="mb-2">
              <p className="text-xs text-muted-foreground mb-1">
                {t("photo.task.item-status")}：{t("photo.task.item-summary", {
                  ok: task.item_status!.filter((it) => it.status === "success").length,
                  failed: task.item_status!.filter((it) => it.status === "failed").length,
                })}
              </p>
              <div className="flex flex-wrap gap-1">
                {task.item_status!.map((it) => (
                  <span key={it.index}
                    title={it.status === "failed" ? friendlyError(it.error, t) : it.filename}
                    className={`px-1.5 py-0.5 rounded text-[10px] border ${
                      it.status === "failed" ? "border-destructive text-destructive" : "border-input text-muted-foreground"
                    }`}>
                    {it.status === "failed" ? "✗" : "✓"} {it.filename || `#${it.index + 1}`}
                  </span>
                ))}
              </div>
            </div>
          )}
          {(task.source_filenames?.length ?? 0) > 0 && (
            <p className="text-muted-foreground text-xs mb-2">{t("photo.task.source-files", { files: task.source_filenames.join(", ") })}</p>
          )}
          {results.length > 0 && (
            <>
              {results.length > 1 && (
                <div className="flex justify-end items-center gap-2 mb-2">
                  {picked.size > 0 && (
                    <>
                      <button className="text-xs text-muted-foreground hover:text-foreground" onClick={() => setPicked(new Set())}>
                        {t("photo.task.pick-clear")}
                      </button>
                      <a href={getDownloadZipUrl(Array.from(picked), zipPrefix)} download>
                        <Button size="sm" variant="default">
                          <Package className="h-3 w-3 mr-1" />{t("photo.task.pick-download", { count: picked.size })}
                        </Button>
                      </a>
                    </>
                  )}
                  <a href={getDownloadZipUrl(results, zipPrefix)} download>
                    <Button size="sm" variant="outline">
                      <Package className="h-3 w-3 mr-1" />{t("photo.task.zip-download", { count: results.length })}
                    </Button>
                  </a>
                </div>
              )}
              <div className="flex flex-wrap gap-2">
                {results.map((url, i) => (
                  <div key={`r-${i}`} className="relative">
                    {/* 多变体挑选：勾选后可仅下载选中（收藏择优） */}
                    <button type="button" onClick={() => togglePick(url)}
                      className={`absolute top-1 left-1 z-10 h-5 w-5 rounded-full border flex items-center justify-center text-[10px] ${
                        picked.has(url) ? "bg-primary text-primary-foreground border-primary" : "bg-background/80 border-input text-transparent"
                      }`}>
                      ✓
                    </button>
                    <ResultPreview url={url} index={i} sourceUrl={sourceFor(i)} onInpaint={onInpaint} />
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
};

const TaskTable: React.FC<Props> = ({ tasks, images = [], onDelete, onRetry, onRefreshTask, onRefreshAll, onInpaint }) => {
  const { t } = useTranslation();
  const [tab, setTab] = React.useState<"active" | "history">("active");

  // 图库 id → url，用于把任务源图解析出来做 before/after 对比
  const urlById = React.useMemo(() => {
    const m = new Map<string, string>();
    images.forEach((img) => m.set(img.id, img.url));
    return m;
  }, [images]);
  const resolveSources = (task: PhotoTask) =>
    (task.image_ids ?? []).map((id) => urlById.get(id)).filter((u): u is string => !!u);

  const activeTasks = tasks.filter((tk) => ["pending", "processing"].includes(tk.status));
  // 历史：失败（含部分成功）置顶，便于优先干预；同组内保持后端的时间倒序
  const historyTasks = tasks
    .filter((tk) => ["success", "failed"].includes(tk.status))
    .sort((a, b) => (a.status === "failed" ? 0 : 1) - (b.status === "failed" ? 0 : 1));

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
        <div className="text-center py-8">
          <p className="text-muted-foreground">{tab === "active" ? t("photo.task.empty-active") : t("photo.task.empty-history")}</p>
          <p className="mt-1 text-xs text-muted-foreground/70">{t("photo.task.empty-hint")}</p>
        </div>
      ) : (
        list.map((task) => (
          <TaskRow key={task.task_id} task={task} sourceUrls={resolveSources(task)}
            onDelete={onDelete} onRetry={onRetry} onRefresh={onRefreshTask} onInpaint={onInpaint} />
        ))
      )}
    </div>
  );
};

export default TaskTable;
