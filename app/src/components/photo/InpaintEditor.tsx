import React, { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button.tsx";
import { Input } from "@/components/ui/input.tsx";
import { Label } from "@/components/ui/label.tsx";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from "@/components/ui/dialog.tsx";
import { Brush, Eraser, Undo2, Trash2 } from "lucide-react";

interface Props {
  imageUrl: string | null;
  open: boolean;
  loading?: boolean;
  onOpenChange: (open: boolean) => void;
  onApply: (maskBase64: string, prompt: string) => void;
}

// 画布内局部重绘。两张离屏画布分离职责：
//  - maskRef    黑底白笔，用于导出（白=重绘区，黑=保留），从不含源图，导出永不被跨域污染
//  - overlayRef 透明底红笔，仅用于在显示画布上做半透明红色预览
const InpaintEditor: React.FC<Props> = ({ imageUrl, open, loading, onOpenChange, onApply }) => {
  const { t } = useTranslation();
  const dispRef = useRef<HTMLCanvasElement>(null);
  const maskRef = useRef<HTMLCanvasElement>(null);
  const overlayRef = useRef<HTMLCanvasElement>(null);
  const imgRef = useRef<HTMLImageElement | null>(null);
  const drawing = useRef(false);
  const history = useRef<{ mask: ImageData; overlay: ImageData }[]>([]);
  const scaleRef = useRef(1);

  const [brush, setBrush] = useState(36);
  const [mode, setMode] = useState<"erase" | "restore">("erase");
  const [prompt, setPrompt] = useState("");
  const [ready, setReady] = useState(false);
  const [hasMask, setHasMask] = useState(false);

  // 显示：源图 + 透明红色 overlay
  const render = useCallback(() => {
    const dc = dispRef.current, ov = overlayRef.current, img = imgRef.current;
    if (!dc || !ov || !img) return;
    const ctx = dc.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, dc.width, dc.height);
    ctx.drawImage(img, 0, 0, dc.width, dc.height);
    ctx.drawImage(ov, 0, 0, dc.width, dc.height);
  }, []);

  useEffect(() => {
    if (!open || !imageUrl) return;
    setReady(false);
    setHasMask(false);
    history.current = [];
    const img = new Image();
    img.crossOrigin = "anonymous";
    img.onload = () => {
      imgRef.current = img;
      const w = img.naturalWidth || img.width, h = img.naturalHeight || img.height;
      const mc = maskRef.current, ov = overlayRef.current, dc = dispRef.current;
      if (!mc || !ov || !dc) return;
      mc.width = w; mc.height = h;
      ov.width = w; ov.height = h;
      const mctx = mc.getContext("2d");
      if (mctx) { mctx.fillStyle = "#000"; mctx.fillRect(0, 0, w, h); }   // 导出底：黑
      ov.getContext("2d")?.clearRect(0, 0, w, h);                          // 预览底：透明
      const s = Math.min(1, 640 / w, 520 / h);
      scaleRef.current = s;
      dc.width = Math.round(w * s); dc.height = Math.round(h * s);
      setReady(true);
      render();
    };
    img.onerror = () => { setReady(false); };
    img.src = imageUrl;
  }, [open, imageUrl, render]);

  const paintAt = (clientX: number, clientY: number) => {
    const dc = dispRef.current, mc = maskRef.current, ov = overlayRef.current;
    if (!dc || !mc || !ov) return;
    const rect = dc.getBoundingClientRect();
    const x = (clientX - rect.left) / scaleRef.current;
    const y = (clientY - rect.top) / scaleRef.current;
    const r = brush / scaleRef.current;
    const mctx = mc.getContext("2d"), octx = ov.getContext("2d");
    if (!mctx || !octx) return;

    // 导出 mask：白=重绘 / 黑=保留
    mctx.globalCompositeOperation = "source-over";
    mctx.fillStyle = mode === "erase" ? "#fff" : "#000";
    mctx.beginPath(); mctx.arc(x, y, r, 0, Math.PI * 2); mctx.fill();

    // 预览 overlay：红=重绘；擦除模式用 destination-out 清掉
    if (mode === "erase") {
      octx.globalCompositeOperation = "source-over";
      octx.fillStyle = "rgba(239,68,68,0.55)";
    } else {
      octx.globalCompositeOperation = "destination-out";
      octx.fillStyle = "rgba(0,0,0,1)";
    }
    octx.beginPath(); octx.arc(x, y, r, 0, Math.PI * 2); octx.fill();
    octx.globalCompositeOperation = "source-over";

    if (mode === "erase") setHasMask(true);
    render();
  };

  const pushHistory = () => {
    const mc = maskRef.current, ov = overlayRef.current;
    const mctx = mc?.getContext("2d"), octx = ov?.getContext("2d");
    if (mc && ov && mctx && octx) {
      history.current.push({
        mask: mctx.getImageData(0, 0, mc.width, mc.height),
        overlay: octx.getImageData(0, 0, ov.width, ov.height),
      });
      if (history.current.length > 20) history.current.shift();
    }
  };

  const onDown = (e: React.PointerEvent) => {
    if (!ready) return;
    pushHistory();
    drawing.current = true;
    (e.target as HTMLElement).setPointerCapture?.(e.pointerId);
    paintAt(e.clientX, e.clientY);
  };
  const onMove = (e: React.PointerEvent) => {
    if (drawing.current) paintAt(e.clientX, e.clientY);
  };
  const onUp = () => { drawing.current = false; };

  const undo = () => {
    const mc = maskRef.current, ov = overlayRef.current;
    const mctx = mc?.getContext("2d"), octx = ov?.getContext("2d");
    const prev = history.current.pop();
    if (mc && ov && mctx && octx && prev) {
      mctx.putImageData(prev.mask, 0, 0);
      octx.putImageData(prev.overlay, 0, 0);
      render();
    }
  };

  const clearMask = () => {
    const mc = maskRef.current, ov = overlayRef.current;
    const mctx = mc?.getContext("2d"), octx = ov?.getContext("2d");
    if (mc && ov && mctx && octx) {
      pushHistory();
      mctx.fillStyle = "#000"; mctx.fillRect(0, 0, mc.width, mc.height);
      octx.clearRect(0, 0, ov.width, ov.height);
      setHasMask(false);
      render();
    }
  };

  const apply = () => {
    const mc = maskRef.current;
    if (!mc || !hasMask) return;
    onApply(mc.toDataURL("image/png"), prompt.trim());
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-1.5 text-base">
            <Brush className="h-4 w-4" /> {t("photo.inpaint.title")}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-3">
          <p className="text-xs text-muted-foreground">{t("photo.inpaint.hint")}</p>

          <div className="flex flex-wrap items-center gap-2">
            <Button size="sm" variant={mode === "erase" ? "default" : "outline"} onClick={() => setMode("erase")}>
              <Brush className="h-3.5 w-3.5 mr-1" />{t("photo.inpaint.brush")}
            </Button>
            <Button size="sm" variant={mode === "restore" ? "default" : "outline"} onClick={() => setMode("restore")}>
              <Eraser className="h-3.5 w-3.5 mr-1" />{t("photo.inpaint.restore")}
            </Button>
            <div className="flex items-center gap-1 text-xs text-muted-foreground">
              {t("photo.inpaint.size")}
              <input type="range" min={8} max={80} value={brush} onChange={(e) => setBrush(Number(e.target.value))} />
            </div>
            <Button size="sm" variant="ghost" onClick={undo} disabled={history.current.length === 0}>
              <Undo2 className="h-3.5 w-3.5 mr-1" />{t("photo.inpaint.undo")}
            </Button>
            <Button size="sm" variant="ghost" onClick={clearMask} disabled={!hasMask}>
              <Trash2 className="h-3.5 w-3.5 mr-1" />{t("photo.inpaint.clear")}
            </Button>
          </div>

          <div className="flex justify-center rounded-md border bg-muted/30 p-2 overflow-auto max-h-[55vh]">
            <canvas
              ref={dispRef}
              className="touch-none cursor-crosshair rounded select-none"
              onPointerDown={onDown}
              onPointerMove={onMove}
              onPointerUp={onUp}
              onPointerCancel={onUp}
            />
            <canvas ref={maskRef} className="hidden" />
            <canvas ref={overlayRef} className="hidden" />
          </div>

          <div>
            <Label className="text-xs">{t("photo.inpaint.prompt")}</Label>
            <Input value={prompt} onChange={(e) => setPrompt(e.target.value)}
              placeholder={t("photo.inpaint.prompt-ph")} className="mt-1" />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>{t("photo.inpaint.cancel")}</Button>
          <Button onClick={apply} disabled={!hasMask || loading}>
            {loading ? t("photo.inpaint.applying") : t("photo.inpaint.apply")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default InpaintEditor;
