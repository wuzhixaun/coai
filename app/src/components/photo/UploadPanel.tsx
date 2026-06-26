import React, { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Button } from "@/components/ui/button.tsx";
import { Progress } from "@/components/ui/progress.tsx";
import { Skeleton } from "@/components/ui/skeleton.tsx";
import { Upload, FolderOpen, Trash2, Star, Sparkles, Link as LinkIcon } from "lucide-react";
import { toast } from "sonner";
import type { PhotoImage } from "@/api/photo";

interface Props {
  images: PhotoImage[];
  imagesLoading?: boolean;
  selectedIds: string[];
  uploading: boolean;
  uploadProgress: number;
  onUpload: (files: File[]) => void;
  onUploadFolder: (files: File[], folderName: string) => void;
  onToggleSelect: (id: string) => void;
  onSelectAll: () => void;
  onClearSelection: () => void;
  onRemove: (id: string) => void;
  onClearAll: () => void;
  onFavorite?: (img: PhotoImage) => void;
  onFetchUrl?: (url: string) => void | Promise<void>;
}

const ALLOWED = ["image/png", "image/jpeg", "image/webp", "image/bmp", "image/tiff"];
const MAX_SIZE = 50 * 1024 * 1024;

// 过滤合法文件，并对被拒绝的文件给出可见提示（类型不符 / 超过 50MB）
function filterValid(files: File[], t: TFunction): File[] {
  const valid: File[] = [];
  let badType = 0;
  let tooBig = 0;
  files.forEach((f) => {
    if (!ALLOWED.includes(f.type)) badType++;
    else if (f.size > MAX_SIZE) tooBig++;
    else valid.push(f);
  });
  if (badType > 0) toast.error(t("photo.upload.bad-type", { count: badType }));
  if (tooBig > 0) toast.error(t("photo.upload.too-big", { count: tooBig }));
  return valid;
}

const UploadPanel: React.FC<Props> = ({
  images, imagesLoading, selectedIds, uploading, uploadProgress, onUpload, onUploadFolder,
  onToggleSelect, onSelectAll, onClearSelection, onRemove, onClearAll, onFavorite, onFetchUrl,
}) => {
  const { t } = useTranslation();
  const fileRef = useRef<HTMLInputElement>(null);
  const folderRef = useRef<HTMLInputElement>(null);
  const [dragOver, setDragOver] = useState(false);
  const [pendingCount, setPendingCount] = useState(0);
  const [urlInput, setUrlInput] = useState("");
  const [fetchingUrl, setFetchingUrl] = useState(false);

  const handleFetchUrl = async () => {
    const u = urlInput.trim();
    if (!u || !onFetchUrl) return;
    setFetchingUrl(true);
    try {
      await onFetchUrl(u);
      setUrlInput("");
    } finally {
      setFetchingUrl(false);
    }
  };

  const handleFiles = (files: File[]) => {
    const valid = filterValid(files, t);
    if (valid.length) {
      setPendingCount(valid.length);
      onUpload(valid);
    }
  };

  // 全局 Ctrl+V 粘贴上传：从剪贴板读取图片文件直接上传
  useEffect(() => {
    const onPaste = (e: ClipboardEvent) => {
      const items = e.clipboardData?.items;
      if (!items) return;
      const files = Array.from(items)
        .filter((it) => it.kind === "file")
        .map((it) => it.getAsFile())
        .filter((f): f is File => !!f);
      if (files.length) {
        e.preventDefault();
        handleFiles(files);
      }
    };
    window.addEventListener("paste", onPaste);
    return () => window.removeEventListener("paste", onPaste);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 上传结束后清掉骨架占位
  useEffect(() => {
    if (!uploading) setPendingCount(0);
  }, [uploading]);

  const handleFolder = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;
    const valid = filterValid(Array.from(files), t);
    if (valid.length) {
      const folderName = valid[0].webkitRelativePath?.split("/")[0] || "";
      setPendingCount(valid.length);
      onUploadFolder(valid, folderName);
    }
    e.target.value = "";
  };

  return (
    <div className="p-4 h-full flex flex-col">
      {/* Drop zone */}
      <div
        className={`border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-all ${
          dragOver
            ? "border-primary bg-primary/10 ring-2 ring-primary/30 scale-[1.01]"
            : "border-input hover:border-primary/50 hover:bg-muted/40"
        }`}
        onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
        onDragLeave={() => setDragOver(false)}
        onDrop={(e) => { e.preventDefault(); setDragOver(false); handleFiles(Array.from(e.dataTransfer.files)); }}
        onClick={() => fileRef.current?.click()}
      >
        <Upload className={`mx-auto h-8 w-8 transition-colors ${dragOver ? "text-primary" : "text-muted-foreground"}`} />
        <p className="mt-2 text-sm text-foreground">
          {dragOver ? t("photo.upload.hint-drop") : t("photo.upload.hint")}
        </p>
        <p className="text-xs text-muted-foreground">{t("photo.upload.formats")}</p>
        <input ref={fileRef} type="file" accept="image/*" multiple className="hidden"
          onChange={(e) => { handleFiles(Array.from(e.target.files || [])); e.target.value = ""; }} />
      </div>

      {/* Folder upload */}
      <input ref={folderRef} type="file" /* @ts-expect-error webkitdirectory */
        webkitdirectory="" multiple className="hidden" onChange={handleFolder} />
      <Button variant="outline" className="mt-2 w-full" onClick={() => folderRef.current?.click()}
        disabled={uploading}>
        <FolderOpen className="mr-2 h-4 w-4" /> {t("photo.upload.folder")}
      </Button>

      {/* 贴链接抓图 */}
      {onFetchUrl && (
        <div className="mt-2 flex gap-1">
          <input
            type="url"
            value={urlInput}
            onChange={(e) => setUrlInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") handleFetchUrl(); }}
            placeholder={t("photo.upload.url-ph")}
            className="flex-1 min-w-0 rounded-md border bg-background px-2 text-sm"
          />
          <Button variant="outline" size="sm" disabled={!urlInput.trim() || fetchingUrl}
            onClick={handleFetchUrl}>
            <LinkIcon className="h-3.5 w-3.5 mr-1" />{t("photo.upload.url-fetch")}
          </Button>
        </div>
      )}

      {/* Upload progress */}
      {uploading && (
        <div className="mt-3">
          <div className="flex justify-between text-xs text-muted-foreground mb-1">
            <span>{pendingCount > 0 ? t("photo.upload.uploading-count", { count: pendingCount }) : t("photo.upload.uploading")}</span>
            <span>{t("photo.upload.progress", { percent: uploadProgress })}</span>
          </div>
          <Progress value={uploadProgress} className="h-2" />
        </div>
      )}

      {/* Selection toolbar */}
      {images.length > 0 && (
        <div className="flex items-center gap-2 mt-3 text-sm">
          <Button size="sm" variant="ghost" onClick={onSelectAll}>{t("photo.upload.select-all")}</Button>
          <Button size="sm" variant="ghost" onClick={onClearSelection}>{t("photo.upload.clear-selection")}</Button>
          <Button size="sm" variant="ghost" className="text-destructive" onClick={onClearAll}><Trash2 className="h-3 w-3 mr-1" />{t("photo.upload.clear-all")}</Button>
          <span className="ml-auto text-muted-foreground">{t("photo.upload.selected", { selected: selectedIds.length, total: images.length })}</span>
        </div>
      )}

      {/* 初始加载图库时的骨架占位（刷新页面后并发拉取 images 期间） */}
      {imagesLoading && images.length === 0 && !uploading && (
        <div className="grid grid-cols-3 gap-2 mt-2 overflow-auto flex-1 content-start items-start auto-rows-min">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={`init-sk-${i}`} className="w-full h-24 rounded" />
          ))}
        </div>
      )}

      {/* 空状态引导：无图时给出清晰的上手指引（不做名不副实的伪动作按钮） */}
      {!imagesLoading && !uploading && images.length === 0 && (
        <div className="mt-4 rounded-md border border-dashed p-3">
          <p className="text-sm font-medium text-foreground">{t("photo.upload.empty-title")}</p>
          <p className="text-xs text-muted-foreground mt-0.5">{t("photo.upload.empty-desc")}</p>
          <p className="mt-2 flex items-start gap-1.5 text-xs text-muted-foreground">
            <Sparkles className="h-3.5 w-3.5 mt-0.5 shrink-0 text-primary" />
            <span>{t("photo.upload.empty-can-do")}</span>
          </p>
        </div>
      )}

      {/* Thumbnail grid */}
      {(images.length > 0 || (uploading && pendingCount > 0)) && (
        <div className="grid grid-cols-3 gap-2 mt-2 overflow-auto flex-1 content-start items-start auto-rows-min">
          {images.map((img) => (
            <div key={img.id} className={`relative rounded border-2 cursor-pointer ${
              selectedIds.includes(img.id) ? "border-primary" : "border-transparent"
            }`} onClick={() => onToggleSelect(img.id)}>
              <img src={img.url} alt={img.filename}
                className="w-full h-24 object-cover rounded bg-muted" />
              <p className="text-[10px] px-1 truncate text-muted-foreground">{img.filename}</p>
              {selectedIds.includes(img.id) && (
                <span className="absolute top-1 right-1 bg-primary text-primary-foreground rounded-full w-4 h-4 flex items-center justify-center text-[10px]">✓</span>
              )}
              <button className="absolute top-1 left-1 bg-destructive text-destructive-foreground rounded-full w-4 h-4 flex items-center justify-center text-[10px]"
                onClick={(e) => { e.stopPropagation(); onRemove(img.id); }}>×</button>
              {onFavorite && (
                <button title={t("photo.upload.favorite")}
                  className="absolute bottom-5 right-1 bg-background/80 text-amber-500 rounded-full w-5 h-5 flex items-center justify-center hover:bg-background"
                  onClick={(e) => { e.stopPropagation(); onFavorite(img); }}>
                  <Star className="h-3 w-3" />
                </button>
              )}
            </div>
          ))}
          {/* Skeleton placeholders while uploading */}
          {uploading && Array.from({ length: Math.min(pendingCount, 6) }).map((_, i) => (
            <Skeleton key={`sk-${i}`} className="w-full h-24 rounded" />
          ))}
        </div>
      )}
    </div>
  );
};

export default UploadPanel;
