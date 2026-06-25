import React, { useEffect, useState, useRef } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button.tsx";
import { Badge } from "@/components/ui/badge.tsx";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog.tsx";
import { Input } from "@/components/ui/input.tsx";
import { Label } from "@/components/ui/label.tsx";
import { getPrompts, uploadImages, type PromptsConfig } from "@/api/photo";
import { useSelector } from "react-redux";
import { selectSupportModels } from "@/store/chat.ts";

interface Props {
  selectedCount: number;
  loading: boolean;
  onProcess: (features: string[], paramsMap: Record<string, Record<string, unknown>>, model: string) => void;
}

// 功能标签文案走 i18n（photo.features.<key>），这里仅保留 key 与图标
const FEATURES: { key: string; icon: string }[] = [
  { key: "white_bg", icon: "⚪" },
  { key: "scene_gen", icon: "🏞️" },
  { key: "image_erase", icon: "🧹" },
  { key: "color_change", icon: "🎨" },
  { key: "marketing", icon: "📢" },
  { key: "image_translate", icon: "🌐" },
  { key: "hd_upscale", icon: "✨" },
  { key: "model_image", icon: "👤" },
  { key: "material_change", icon: "🪨" },
  { key: "instruction_gen", icon: "📝" },
  { key: "detail_image", icon: "🔍" },
  { key: "logo_custom", icon: "🏷️" },
  { key: "production_flow", icon: "📊" },
  { key: "resize", icon: "📐" },
  { key: "material_extract", icon: "🧩" },
  { key: "product_extract", icon: "📦" },
  { key: "video_gen", icon: "🎬" },
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

// 生图模型下拉只展示市场里打了「绘图」(image-generation) 标签的模型，并使用市场配置的
// 中文友好名。这样以后接入 GPT 画图 / 千问(通义万相) 等模型，只要在后台给它打上该标签
// 就会自动出现；聊天模型(未打标签)与能力模型(superres/extract/video，不在市场列表)都不会混入，
// 避免电商同事误选。
const IMAGE_GEN_TAG = "image-generation";

const FeaturePanel: React.FC<Props> = ({ selectedCount, loading, onProcess }) => {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [dialogKey, setDialogKey] = useState<string | null>(null);
  const [params, setParams] = useState<Record<string, Record<string, unknown>>>({});
  const [prompts, setPrompts] = useState<PromptsConfig | null>(null);
  const [chosenModel, setChosenModel] = useState<string>("");
  const [imageCount, setImageCount] = useState(1);
  const [logoUploading, setLogoUploading] = useState(false);
  const [logoName, setLogoName] = useState("");
  const logoInputRef = useRef<HTMLInputElement>(null);
  const mounted = useRef(true);

  // 从市场模型中按「绘图」标签筛出生图模型（可扩展：新模型打标签即可出现）
  const supportModels = useSelector(selectSupportModels);
  const genModels = supportModels.filter((m) => (m.tag || []).includes(IMAGE_GEN_TAG));

  useEffect(() => {
    mounted.current = true;
    getPrompts().then((data) => { if (mounted.current) setPrompts(data); }).catch(() => {});
    return () => { mounted.current = false; };
  }, []);

  // 市场模型加载后，默认选中第一个生图模型
  useEffect(() => {
    if (!chosenModel && genModels.length > 0) setChosenModel(genModels[0].id);
  }, [genModels, chosenModel]);

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
      if (key === "video_gen") init.duration = 5;
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

  // Logo 定制：直接上传 Logo 文件，拿到图片 id 写入 logo_image_id，省去手填 id。
  const handleLogoUpload = async (file?: File) => {
    if (!file) return;
    setLogoUploading(true);
    try {
      const imgs = await uploadImages([file], "logo");
      if (imgs.length > 0) {
        setParam("logo_image_id", imgs[0].id);
        setLogoName(file.name);
      }
    } catch {
      setLogoName("");
    } finally {
      setLogoUploading(false);
      if (logoInputRef.current) logoInputRef.current.value = "";
    }
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
      <h3 className="font-medium mb-3">{t("photo.feature.title")}</h3>

      {/* Model Selector */}
      <div className="mb-3">
        <Label>{t("photo.feature.model")}</Label>
        <select className="w-full border rounded p-2 mt-1 text-sm bg-background"
          value={chosenModel}
          onChange={(e) => setChosenModel(e.target.value)}>
          {genModels.map((m) => (
            <option key={m.id} value={m.id}>{m.name || m.id}</option>
          ))}
        </select>
      </div>

      <div className="mb-3">
        <Label>{t("photo.feature.count")}</Label>
        <Input
          className="mt-1"
          type="number"
          min={1}
          max={6}
          value={normalizedImageCount}
          onChange={(e) => setImageCount(Number(e.target.value))}
        />
        <p className="text-xs text-muted-foreground mt-1">
          {t("photo.feature.count-hint")}
        </p>
      </div>

      <div className="flex flex-wrap gap-2 mb-4">
        {FEATURES.map((f) => (
          <Button key={f.key} size="sm" variant={selected.has(f.key) ? "default" : "outline"}
            onClick={() => toggle(f.key)}>
            {f.icon} {t(`photo.features.${f.key}`)}
          </Button>
        ))}
      </div>

      <Button className="w-full" onClick={handleBatchProcess}
        disabled={selectedCount === 0 || selected.size === 0 || loading}>
        {loading ? t("photo.feature.processing") : t("photo.feature.process", { count: selected.size })}
      </Button>

      <Dialog open={!!dialogKey} onOpenChange={(open) => { if (!open) setDialogKey(null); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("photo.feature.params-title", { feature: dialogKey ? t(`photo.features.${dialogKey}`) : "" })}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3" style={{ maxHeight: "60vh", overflow: "auto" }}>
            {/* Templates */}
            {(cfg?.templates?.length ?? 0) > 0 && (
              <div>
                <Label>{t("photo.feature.templates")}</Label>
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
                <Label>{dialogKey === "video_gen" ? t("photo.feature.video-prompt") : t("photo.feature.prompt")}</Label>
                <Input value={(p.prompt as string) || ""}
                  onChange={(e) => setParam("prompt", e.target.value)}
                  placeholder={dialogKey === "video_gen" ? t("photo.feature.video-prompt-ph") : t("photo.feature.prompt-ph")} />
              </div>
            )}

            {/* 视频时长 */}
            {dialogKey === "video_gen" && (
              <div>
                <Label>{t("photo.feature.video-duration")}</Label>
                <div className="flex gap-1 mt-1">
                  {[5, 10].map((sec) => (
                    <Badge key={sec} variant={(Number(p.duration) || 5) === sec ? "default" : "outline"}
                      className="cursor-pointer"
                      onClick={() => setParam("duration", sec)}>{t("photo.feature.seconds", { n: sec })}</Badge>
                  ))}
                </div>
              </div>
            )}

            {/* Color picker */}
            {dialogKey === "color_change" && (
              <div>
                <Label>{t("photo.feature.target-color")}</Label>
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
                <Label>{t("photo.feature.target-lang")}</Label>
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
                <Label>{t("photo.feature.selling-point")}</Label>
                <Input value={(p.selling_point as string) || ""}
                  onChange={(e) => setParam("selling_point", e.target.value)} placeholder={t("photo.feature.selling-point-ph")} />
              </div>
            )}

            {/* Logo params */}
            {dialogKey === "logo_custom" && (
              <>
                <div>
                  <Label>{t("photo.feature.logo-image")}</Label>
                  <input ref={logoInputRef} type="file" accept="image/*" className="hidden"
                    onChange={(e) => handleLogoUpload(e.target.files?.[0])} />
                  <div className="flex items-center gap-2 mt-1">
                    <Button type="button" variant="outline" size="sm" disabled={logoUploading}
                      onClick={() => logoInputRef.current?.click()}>
                      {logoUploading ? t("photo.feature.logo-uploading") : (p.logo_image_id ? t("photo.feature.logo-reupload") : t("photo.feature.logo-upload"))}
                    </Button>
                    {p.logo_image_id ? (
                      <span className="text-xs text-green-600 truncate max-w-[180px]">
                        ✓ {logoName || t("photo.feature.logo-uploaded")}
                      </span>
                    ) : (
                      <span className="text-xs text-muted-foreground">{t("photo.feature.logo-hint")}</span>
                    )}
                  </div>
                </div>
                <div>
                  <Label>{t("photo.feature.position")}</Label>
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
                <Label>{t("photo.feature.category")}</Label>
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
                <Label>{t("photo.feature.target-size")}</Label>
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
            <Button variant="outline" onClick={() => setDialogKey(null)}>{t("photo.feature.cancel")}</Button>
            <Button onClick={handleDialogOk}
              disabled={logoUploading || (dialogKey === "logo_custom" && !p.logo_image_id)}>
              {t("photo.feature.start")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default FeaturePanel;
