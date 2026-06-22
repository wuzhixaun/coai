import React, { useEffect, useState, useRef } from "react";
import { Button } from "@/components/ui/button.tsx";
import { Badge } from "@/components/ui/badge.tsx";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog.tsx";
import { Input } from "@/components/ui/input.tsx";
import { Label } from "@/components/ui/label.tsx";
import { getPrompts, type PromptsConfig } from "@/api/photo";
import axios from "axios";

interface Props {
  selectedCount: number;
  loading: boolean;
  onProcess: (features: string[], paramsMap: Record<string, Record<string, unknown>>, model: string) => void;
}

const FEATURES: { key: string; label: string; icon: string }[] = [
  { key: "white_bg", label: "白底图", icon: "⚪" },
  { key: "scene_gen", label: "场景图", icon: "🏞️" },
  { key: "image_erase", label: "擦除", icon: "🧹" },
  { key: "color_change", label: "换色", icon: "🎨" },
  { key: "marketing", label: "营销图", icon: "📢" },
  { key: "image_translate", label: "翻译", icon: "🌐" },
  { key: "hd_upscale", label: "高清", icon: "✨" },
  { key: "model_image", label: "模特图", icon: "👤" },
  { key: "material_change", label: "换材质", icon: "🪨" },
  { key: "instruction_gen", label: "指令生图", icon: "📝" },
  { key: "detail_image", label: "细节图", icon: "🔍" },
  { key: "logo_custom", label: "Logo定制", icon: "🏷️" },
  { key: "production_flow", label: "流程图", icon: "📊" },
  { key: "resize", label: "改尺寸", icon: "📐" },
  { key: "material_extract", label: "素材提取", icon: "🧩" },
  { key: "product_extract", label: "商品提取", icon: "📦" },
  { key: "video_gen", label: "视频", icon: "🎬" },
];

const NEEDS_PARAM: Record<string, string[]> = {
  scene_gen: ["prompt"], image_erase: ["prompt"], color_change: ["target_color"],
  marketing: ["selling_point"], image_translate: ["target_lang"],
  model_image: ["prompt"], instruction_gen: ["prompt"],
  logo_custom: ["logo_image_id", "position"], resize: ["target_sizes"],
  material_extract: ["category"], product_extract: ["category"],
  video_gen: ["prompt"],
};

const COLOR_OPTS = ["red", "blue", "green", "black", "white", "yellow", "purple", "pink", "orange", "gray"];
const LANG_OPTS = ["en", "zh", "ja", "ko", "fr", "de", "es"];
const POS_OPTS = ["bottom-right", "bottom-left", "top-right", "top-left", "center"];
const SIZE_OPTS = ["1:1", "16:9", "4:3", "3:4", "9:16"];

const FeaturePanel: React.FC<Props> = ({ selectedCount, loading, onProcess }) => {
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [dialogKey, setDialogKey] = useState<string | null>(null);
  const [params, setParams] = useState<Record<string, Record<string, unknown>>>({});
  const [prompts, setPrompts] = useState<PromptsConfig | null>(null);
  const [models, setModels] = useState<string[]>([]);
  const [chosenModel, setChosenModel] = useState<string>("");
  const [imageCount, setImageCount] = useState(1);
  const mounted = useRef(true);

  useEffect(() => {
    mounted.current = true;
    getPrompts().then((data) => { if (mounted.current) setPrompts(data); }).catch(() => {});
    // get available models
    axios.get("/v1/models").then((res) => {
      if (mounted.current && res.data?.data) {
        const ids = res.data.data.map((m: any) => typeof m === "string" ? m : m.id);
        setModels(ids);
        if (ids.length > 0 && !chosenModel) setChosenModel(ids[0]);
      }
    }).catch(() => {});
    return () => { mounted.current = false; };
  }, []);

  const toggle = (key: string) => setSelected((prev) => {
    const next = new Set(prev);
    next.has(key) ? next.delete(key) : next.add(key);
    return next;
  });

  const handleFeatureClick = (key: string) => {
    if (selectedCount === 0) return;
    if (!NEEDS_PARAM[key]) { onProcess([key], withImageCount([key]), chosenModel); return; }
    setDialogKey(key);
    if (!params[key]) {
      const init: Record<string, unknown> = {};
      if (key === "color_change") init.target_color = "red";
      if (key === "image_translate") init.target_lang = "en";
      if (key === "marketing") init.selling_point = "Premium Quality";
      if (key === "resize") init.target_sizes = ["1:1", "16:9"];
      if (key === "logo_custom") init.position = "bottom-right";
      if (key === "material_extract") init.category = "提取图案";
      if (key === "product_extract") init.category = "服装";
      setParams((p) => ({ ...p, [key]: init }));
    }
  };

  const handleBatchProcess = () => {
    if (selectedCount === 0 || selected.size === 0) return;
    const needParams = Array.from(selected).filter((k) => NEEDS_PARAM[k]);
    if (needParams.length > 0) { handleFeatureClick(needParams[0]); return; }
    const features = Array.from(selected);
    onProcess(features, withImageCount(features), chosenModel);
    setSelected(new Set());
  };

  const handleDialogOk = () => {
    if (!dialogKey) return;
    onProcess([dialogKey], withImageCount([dialogKey], { [dialogKey]: params[dialogKey] || {} }), chosenModel);
    setDialogKey(null);
  };

  const setParam = (field: string, value: unknown) => {
    if (!dialogKey) return;
    setParams((prev) => ({ ...prev, [dialogKey]: { ...(prev[dialogKey] || {}), [field]: value } }));
  };

  const normalizedImageCount = Math.min(6, Math.max(1, Number.isFinite(imageCount) ? imageCount : 1));

  const withImageCount = (
    features: string[],
    source: Record<string, Record<string, unknown>> = {},
  ): Record<string, Record<string, unknown>> => {
    const next: Record<string, Record<string, unknown>> = {};
    features.forEach((feature) => {
      next[feature] = { ...(source[feature] || {}), image_count: normalizedImageCount };
    });
    return next;
  };

  const cfg = dialogKey ? prompts?.features[dialogKey] : null;
  const p = dialogKey ? (params[dialogKey] || {}) : {};

  return (
    <div className="p-4">
      <h3 className="font-medium mb-3">AI 图片处理功能</h3>

      {/* Model Selector */}
      <div className="mb-3">
        <Label>选择模型</Label>
        <select className="w-full border rounded p-2 mt-1 text-sm bg-background"
          value={chosenModel}
          onChange={(e) => setChosenModel(e.target.value)}>
          {models.map((m) => (
            <option key={m} value={m}>{m}</option>
          ))}
        </select>
      </div>

      <div className="mb-3">
        <Label>生成数量</Label>
        <Input
          className="mt-1"
          type="number"
          min={1}
          max={6}
          value={normalizedImageCount}
          onChange={(e) => setImageCount(Number(e.target.value))}
        />
        <p className="text-xs text-muted-foreground mt-1">
          仅生成类功能生效；高清、改尺寸、视频和本地处理不会重复执行。
        </p>
      </div>

      <div className="flex flex-wrap gap-2 mb-4">
        {FEATURES.map((f) => (
          <Button key={f.key} size="sm" variant={selected.has(f.key) ? "default" : "outline"}
            onClick={() => toggle(f.key)}>
            {f.icon} {f.label}
          </Button>
        ))}
      </div>

      <Button className="w-full" onClick={handleBatchProcess}
        disabled={selectedCount === 0 || selected.size === 0 || loading}>
        {loading ? "处理中..." : `开始处理 (${selected.size} 功能)`}
      </Button>

      <Dialog open={!!dialogKey} onOpenChange={(open) => { if (!open) setDialogKey(null); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{dialogKey ? FEATURES.find((f) => f.key === dialogKey)?.label : ""} 参数设置</DialogTitle>
          </DialogHeader>
          <div className="space-y-3" style={{ maxHeight: "60vh", overflow: "auto" }}>
            {/* Templates */}
            {(cfg?.templates?.length ?? 0) > 0 && (
              <div>
                <Label>快捷模板</Label>
                <div className="flex flex-wrap gap-1 mt-1">
                  {cfg!.templates!.map((t, i) => (
                    <Badge key={i} variant="secondary" className="cursor-pointer"
                      onClick={() => setParam("prompt", t.prompt)}>{t.label}</Badge>
                  ))}
                </div>
              </div>
            )}

            {/* Prompt input */}
            {dialogKey && ["scene_gen", "image_erase", "model_image", "instruction_gen", "video_gen"].includes(dialogKey) && (
              <div>
                <Label>{dialogKey === "video_gen" ? "视频描述（可选）" : "提示词"}</Label>
                <Input value={(p.prompt as string) || ""}
                  onChange={(e) => setParam("prompt", e.target.value)}
                  placeholder={dialogKey === "video_gen" ? "留空则AI自动推理" : "输入提示词..."} />
              </div>
            )}

            {/* Color picker */}
            {dialogKey === "color_change" && (
              <div>
                <Label>目标颜色</Label>
                <div className="flex flex-wrap gap-1 mt-1">
                  {COLOR_OPTS.map((c) => (
                    <Badge key={c} variant={p.target_color === c ? "default" : "outline"} className="cursor-pointer"
                      onClick={() => setParam("target_color", c)}>{c}</Badge>
                  ))}
                </div>
              </div>
            )}

            {/* Language picker */}
            {dialogKey === "image_translate" && (
              <div>
                <Label>目标语言</Label>
                <div className="flex flex-wrap gap-1 mt-1">
                  {LANG_OPTS.map((l) => (
                    <Badge key={l} variant={p.target_lang === l ? "default" : "outline"} className="cursor-pointer"
                      onClick={() => setParam("target_lang", l)}>{l}</Badge>
                  ))}
                </div>
              </div>
            )}

            {/* Selling point */}
            {dialogKey === "marketing" && (
              <div>
                <Label>营销卖点</Label>
                <Input value={(p.selling_point as string) || ""}
                  onChange={(e) => setParam("selling_point", e.target.value)} placeholder="输入营销卖点" />
              </div>
            )}

            {/* Logo params */}
            {dialogKey === "logo_custom" && (
              <>
                <div>
                  <Label>Logo 图片 ID</Label>
                  <Input value={(p.logo_image_id as string) || ""}
                    onChange={(e) => setParam("logo_image_id", e.target.value)} />
                </div>
                <div>
                  <Label>位置</Label>
                  <div className="flex flex-wrap gap-1 mt-1">
                    {POS_OPTS.map((pos) => (
                      <Badge key={pos} variant={p.position === pos ? "default" : "outline"} className="cursor-pointer"
                        onClick={() => setParam("position", pos)}>{pos}</Badge>
                    ))}
                  </div>
                </div>
              </>
            )}

            {/* Category picker (素材/商品提取) */}
            {dialogKey && ["material_extract", "product_extract"].includes(dialogKey) && (
              <div>
                <Label>提取类别</Label>
                <div className="flex flex-wrap gap-1 mt-1">
                  {(cfg?.categories ?? []).map((opt) => (
                    <Badge key={opt.value} variant={p.category === opt.value ? "default" : "outline"} className="cursor-pointer"
                      onClick={() => setParam("category", opt.value)}>{opt.label}</Badge>
                  ))}
                </div>
              </div>
            )}

            {/* Size picker */}
            {dialogKey === "resize" && (
              <div>
                <Label>目标尺寸</Label>
                <div className="flex flex-wrap gap-1 mt-1">
                  {SIZE_OPTS.map((s) => {
                    const cur = (p.target_sizes as string[]) || [];
                    const checked = cur.includes(s);
                    return (
                      <Badge key={s} variant={checked ? "default" : "outline"} className="cursor-pointer"
                        onClick={() => setParam("target_sizes", checked ? cur.filter((x) => x !== s) : [...cur, s])}>
                        {s}
                      </Badge>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogKey(null)}>取消</Button>
            <Button onClick={handleDialogOk}>开始处理</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default FeaturePanel;
