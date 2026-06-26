import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button.tsx";
import { Input } from "@/components/ui/input.tsx";
import { Textarea } from "@/components/ui/textarea.tsx";
import { Label } from "@/components/ui/label.tsx";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogTrigger,
} from "@/components/ui/dialog.tsx";
import { Fingerprint, Palette, Plus, X } from "lucide-react";
import { toast } from "sonner";
import type { PhotoIdentity, PhotoImage } from "@/api/photo";

type IdentityType = "product" | "model" | "brandkit";

interface Props {
  identities: PhotoIdentity[];
  selectedIdentityId: string;
  selectedBrandKitId: string;
  selectedImageIds: string[]; // 当前左侧已选图片，作为新建身份的参考图/Logo 来源
  images: PhotoImage[];
  onSelect: (id: string) => void;
  onSelectBrandKit: (id: string) => void;
  onCreate: (body: { type: string; name: string; ref_image_ids: string[]; subject_prompt?: string; color?: string }) => Promise<unknown>;
  onDelete: (id: string) => void;
}

const IdentityPanel: React.FC<Props> = ({
  identities, selectedIdentityId, selectedBrandKitId, selectedImageIds, images,
  onSelect, onSelectBrandKit, onCreate, onDelete,
}) => {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [type, setType] = useState<IdentityType>("product");
  const [subject, setSubject] = useState("");
  const [color, setColor] = useState("#000000");
  const [submitting, setSubmitting] = useState(false);
  const [filter, setFilter] = useState<"all" | "product" | "model">("all");

  const consistencyIdentities = identities.filter((it) => it.type === "product" || it.type === "model");
  const brandKits = identities.filter((it) => it.type === "brandkit");
  const visibleIdentities = consistencyIdentities.filter((it) => filter === "all" || it.type === filter);

  const openDialog = (initialType: IdentityType) => {
    setType(initialType);
    setOpen(true);
  };

  const handleCreate = async () => {
    if (selectedImageIds.length === 0) {
      toast.error(t("photo.identity.no-refs"));
      return;
    }
    if (!name.trim()) return;
    setSubmitting(true);
    try {
      // brandkit：取已选第一张作为 Logo；商品/模特：全部已选作为参考图
      const refIds = type === "brandkit" ? [selectedImageIds[0]] : selectedImageIds;
      await onCreate({
        type, name: name.trim(), ref_image_ids: refIds,
        subject_prompt: type === "brandkit" ? "" : subject.trim(),
        color: type === "brandkit" ? color : "",
      });
      setOpen(false);
      setName("");
      setSubject("");
      setColor("#000000");
      setType("product");
    } catch {
      toast.error(t("photo.identity.no-refs"));
    } finally {
      setSubmitting(false);
    }
  };

  const chip = (it: PhotoIdentity, selected: boolean, onPick: (id: string) => void) => (
    <span
      key={it.id}
      className={`group flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs transition-colors ${
        selected ? "border-primary bg-primary/10 text-primary" : "border-input text-foreground hover:bg-muted/40"
      }`}
    >
      <button type="button" onClick={() => onPick(it.id)} className="flex items-center gap-1">
        {it.type === "brandkit" && it.color && (
          <span className="inline-block h-2.5 w-2.5 rounded-full border" style={{ backgroundColor: it.color }} />
        )}
        {it.name}
      </button>
      <button type="button" title={t("photo.identity.delete")} onClick={() => onDelete(it.id)}
        className="opacity-0 group-hover:opacity-100 transition-opacity text-destructive">
        <X className="h-3 w-3" />
      </button>
    </span>
  );

  return (
    <div className="border-b bg-card px-4 py-2 space-y-1.5">
      {/* 一致性身份：商品/模特 */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
          <Fingerprint className="h-3.5 w-3.5" /> {t("photo.identity.title")}
        </span>

        {consistencyIdentities.length > 0 && (
          <div className="flex items-center gap-0.5 rounded-md border p-0.5">
            {(["all", "product", "model"] as const).map((f) => (
              <button key={f} type="button" onClick={() => setFilter(f)}
                className={`rounded px-1.5 py-0.5 text-[11px] transition-colors ${
                  filter === f ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-muted/40"
                }`}>
                {f === "all" ? t("photo.identity.all") : f === "product" ? t("photo.identity.type-product") : t("photo.identity.type-model")}
              </button>
            ))}
          </div>
        )}

        <button type="button" onClick={() => onSelect("")}
          className={`rounded-full border px-2.5 py-0.5 text-xs transition-colors ${
            selectedIdentityId === "" ? "border-primary bg-primary/10 text-primary" : "border-input text-muted-foreground hover:bg-muted/40"
          }`}>
          {t("photo.identity.none")}
        </button>

        {visibleIdentities.map((it) => chip(it, selectedIdentityId === it.id, onSelect))}

        <Button size="sm" variant="outline" className="h-6 px-2 text-xs" onClick={() => openDialog("product")}>
          <Plus className="h-3 w-3 mr-1" /> {t("photo.identity.new")}
        </Button>
      </div>

      {/* 品牌资产：与一致性身份可组合 */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
          <Palette className="h-3.5 w-3.5" /> {t("photo.identity.brand-title")}
        </span>

        <button type="button" onClick={() => onSelectBrandKit("")}
          className={`rounded-full border px-2.5 py-0.5 text-xs transition-colors ${
            selectedBrandKitId === "" ? "border-primary bg-primary/10 text-primary" : "border-input text-muted-foreground hover:bg-muted/40"
          }`}>
          {t("photo.identity.none")}
        </button>

        {brandKits.map((it) => chip(it, selectedBrandKitId === it.id, onSelectBrandKit))}

        <Button size="sm" variant="outline" className="h-6 px-2 text-xs" onClick={() => openDialog("brandkit")}>
          <Plus className="h-3 w-3 mr-1" /> {t("photo.identity.brand-new")}
        </Button>
      </div>

      {(selectedIdentityId || selectedBrandKitId) && (
        <p className="text-[10px] text-muted-foreground">{t("photo.identity.hint")}</p>
      )}

      {/* 新建对话框（商品/模特/品牌共用） */}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger asChild><span className="hidden" /></DialogTrigger>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-1.5 text-base">
              <Fingerprint className="h-4 w-4" /> {type === "brandkit" ? t("photo.identity.brand-new") : t("photo.identity.new")}
            </DialogTitle>
          </DialogHeader>

          <div className="space-y-3">
            <div className="flex gap-2">
              {(["product", "model", "brandkit"] as const).map((tp) => (
                <Button key={tp} size="sm" variant={type === tp ? "default" : "outline"} onClick={() => setType(tp)}>
                  {tp === "product" ? t("photo.identity.type-product") : tp === "model" ? t("photo.identity.type-model") : t("photo.identity.type-brandkit")}
                </Button>
              ))}
            </div>

            <div>
              <Label className="text-xs">{t("photo.identity.name")}</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder={t("photo.identity.name-ph")} className="mt-1" />
            </div>

            {type === "brandkit" ? (
              <div>
                <Label className="text-xs">{t("photo.identity.color")}</Label>
                <div className="mt-1 flex items-center gap-2">
                  <input type="color" value={color} onChange={(e) => setColor(e.target.value)} className="h-8 w-12 rounded border bg-transparent" />
                  <Input value={color} onChange={(e) => setColor(e.target.value)} className="w-28" />
                </div>
              </div>
            ) : (
              <div>
                <Label className="text-xs">{t("photo.identity.subject")}</Label>
                <Textarea value={subject} onChange={(e) => setSubject(e.target.value)} placeholder={t("photo.identity.subject-ph")} className="mt-1" rows={2} />
              </div>
            )}

            <div>
              <Label className="text-xs">{type === "brandkit" ? t("photo.identity.logo") : t("photo.identity.refs")}</Label>
              {selectedImageIds.length === 0 ? (
                <p className="mt-1 text-xs text-destructive">{t("photo.identity.no-refs")}</p>
              ) : (
                <div className="mt-1 flex flex-wrap gap-1">
                  {(type === "brandkit" ? selectedImageIds.slice(0, 1) : selectedImageIds).map((id) => {
                    const img = images.find((x) => x.id === id);
                    return img ? (
                      <img key={id} src={img.url} alt={img.filename} className="h-12 w-12 rounded border object-cover" />
                    ) : null;
                  })}
                </div>
              )}
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>{t("photo.identity.cancel")}</Button>
            <Button onClick={handleCreate} disabled={submitting || !name.trim() || selectedImageIds.length === 0}>
              {t("photo.identity.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
};

export default IdentityPanel;
