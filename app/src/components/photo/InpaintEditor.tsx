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

// 画布内局部重绘：源图画在显示画布(仅预览)，mask 画在独立离屏画布(黑底白笔，仅导出它)。
// 这样即便源图跨域污染显示画布，mask 导出始终干净。
const InpaintEditor: React.FC<Props> = ({ imageUrl, open, loading, onOpenChange, onApply }) => {
  const { t } = useTranslation();
  const dispRef = useRef<HTMLCanvasElement>(null);
  const maskRef = useRef<HTMLCanvasElement>(null);   // 离屏：自然尺寸，黑底白笔
  const imgRef = useRef<HTMLImageElement | null>(null);
  const drawing = useRef(false);
  const history = useRef<ImageData[]>([]);
  const scaleRef = useRef(1);

  const [brush, setBrush] = useState(36);
  const [mode, setMode] = useState<"erase" | "restore">("erase");
  const [prompt, setPrompt] = useState("");
  const [ready, setReady] = useState(false);
  const [hasMask, setHasMask] = useState(false);

  // 把 mask(白) 以半透明红叠加到显示画布上
  const render = useCallback(() => {
    const dc = dispRef.current, mc = maskRef.current, img = imgRef.current;
    if (!dc || !mc || !img) return;
    const ctx = dc.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, dc.width, dc.height);
    ctx.drawImage(img, 0, 0, dc.width, dc.height);
    const tmp = document.createElement("canvas");
    tmp.width = dc.width; tmp.height = dc.height;
    const tctx = tmp.getContext("2d");
    if (tctx) {
      tctx.drawImage(mc, 0, 0, dc.width, dc.height);
      tctx.globalCompositeOperation = "source-in";
      tctx.fillStyle = "rgba(239,68,68,1)";
      tctx.fillRect(0, 0, dc.width, dc.height);
      ctx.globalAlpha = 0.5;
      ctx.drawImage(tmp, 0, 0);
      ctx.globalAlpha = 1;
    }
  }, []);

  // 载入图片，初始化画布
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
      const mc = maskRef.current, dc = dispRef.current;
      if (!mc || !dc) return;
      mc.width = w; mc.height = h;
      const mctx = mc.getContext("2d");
      if (mctx) { mctx.fillStyle = "#000"; mctx.fillRect(0, 0, w, h); }
      const s = Math.min(1, 640 / w, 520 / h);
      scaleRef.current = s;
      dc.width = Math.round(w * s); dc.height = Math.round(h * s);
      setReady(true);
      render();
    };
    img.src = imageUrl;
  }, [open, imageUrl, render]);

  const paintAt = (clientX: number, clientY: number) => {
    const dc = dispRef.current, mc = maskRef.current;
    if (!dc || !mc) return;
    const rect = dc.getBoundingClientRect();
    const x = (clientX - rect.left) / scaleRef.current;
    const y = (clientY - rect.top) / scaleRef.current;
    const r = brush / scaleRef.current;
    const mctx = mc.getContext("2d");
    if (!mctx) return;
    mctx.globalCompositeOperation = "source-over";
    mctx.fillStyle = mode === "erase" ? "#fff" : "#000";
    mctx.beginPath();
    mctx.arc(x, y, r, 0, Math.PI * 2);
    mctx.fill();
    if (mode === "erase") setHasMask(true);
    render();
  };

  const pushHistory = () => {
    const mc = maskRef.current;
    const mctx = mc?.getContext("2d");
    if (mc && mctx) {
      history.current.push(mctx.getImageData(0, 0, mc.width, mc.height));
      if (history.current.length > 20) history.current.shift();
    }
  };

  const onDown = (e: React.PointerEvent) => {
    if (!ready) return;
    pushHistory();
    drawing.current = true;
    paintAt(e.clientX, e.clientY);
  };
  const onMove = (e: React.PointerEvent) => {
    if (drawing.current) paintAt(e.clientX, e.clientY);
  };
  const onUp = () => { drawing.current = false; };

  const undo = () => {
    const mc = maskRef.current;
    const mctx = mc?.getContext("2d");
    const prev = history.current.pop();
    if (mc && mctx && prev) { mctx.putImageData(prev, 0, 0); render(); }
  };

  const clearMask = () => {
    const mc = maskRef.current;
    const mctx = mc?.getContext("2d");
    if (mc && mctx) {
      pushHistory();
      mctx.fillStyle = "#000"; mctx.fillRect(0, 0, mc.width, mc.height);
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
              className="touch-none cursor-crosshair rounded"
              onPointerDown={onDown}
              onPointerMove={onMove}
              onPointerUp={onUp}
              onPointerLeave={onUp}
            />
            <canvas ref={maskRef} className="hidden" />
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
