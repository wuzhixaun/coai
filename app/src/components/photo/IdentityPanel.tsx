import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button.tsx";
import { Input } from "@/components/ui/input.tsx";
import { Textarea } from "@/components/ui/textarea.tsx";
import { Label } from "@/components/ui/label.tsx";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogTrigger,
} from "@/components/ui/dialog.tsx";
import { Fingerprint, Plus, X } from "lucide-react";
import { toast } from "sonner";
import type { PhotoIdentity, PhotoImage } from "@/api/photo";

interface Props {
  identities: PhotoIdentity[];
  selectedIdentityId: string;
  selectedImageIds: string[]; // 当前左侧已选图片，作为新建身份的参考图来源
  images: PhotoImage[];
  onSelect: (id: string) => void;
  onCreate: (body: { type: string; name: string; ref_image_ids: string[]; subject_prompt?: string }) => Promise<unknown>;
  onDelete: (id: string) => void;
}

const IdentityPanel: React.FC<Props> = ({
  identities, selectedIdentityId, selectedImageIds, images, onSelect, onCreate, onDelete,
}) => {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [type, setType] = useState<"product" | "model">("product");
  const [subject, setSubject] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [filter, setFilter] = useState<"all" | "product" | "model">("all");

  const visibleIdentities = identities.filter((it) => filter === "all" || it.type === filter);

  const handleCreate = async () => {
    if (selectedImageIds.length === 0) {
      toast.error(t("photo.identity.no-refs"));
      return;
    }
    if (!name.trim()) return;
    setSubmitting(true);
    try {
      await onCreate({ type, name: name.trim(), ref_image_ids: selectedImageIds, subject_prompt: subject.trim() });
      setOpen(false);
      setName("");
      setSubject("");
      setType("product");
    } catch {
      toast.error(t("photo.identity.no-refs"));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="border-b bg-card px-4 py-2">
      <div className="flex items-center gap-2 flex-wrap">
        <span className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
          <Fingerprint className="h-3.5 w-3.5" /> {t("photo.identity.title")}
        </span>

        {/* 类型筛选：全部 / 商品 / 模特 */}
        {identities.length > 0 && (
          <div className="flex items-center gap-0.5 rounded-md border p-0.5">
            {(["all", "product", "model"] as const).map((f) => (
              <button
                key={f}
                type="button"
                onClick={() => setFilter(f)}
                className={`rounded px-1.5 py-0.5 text-[11px] transition-colors ${
                  filter === f ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-muted/40"
                }`}
              >
                {f === "all" ? t("photo.identity.all") : f === "product" ? t("photo.identity.type-product") : t("photo.identity.type-model")}
              </button>
            ))}
          </div>
        )}

        {/* 不应用 */}
        <button
          type="button"
          onClick={() => onSelect("")}
          className={`rounded-full border px-2.5 py-0.5 text-xs transition-colors ${
            selectedIdentityId === "" ? "border-primary bg-primary/10 text-primary" : "border-input text-muted-foreground hover:bg-muted/40"
          }`}
        >
          {t("photo.identity.none")}
        </button>

        {/* 身份芯片 */}
        {visibleIdentities.map((it) => (
          <span
            key={it.id}
            className={`group flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs transition-colors ${
              selectedIdentityId === it.id ? "border-primary bg-primary/10 text-primary" : "border-input text-foreground hover:bg-muted/40"
            }`}
          >
            <button type="button" onClick={() => onSelect(it.id)} className="flex items-center gap-1">
              {it.name}
              <span className="text-[10px] text-muted-foreground">
                {it.type === "model" ? t("photo.identity.type-model") : t("photo.identity.type-product")}
              </span>
            </button>
            <button
              type="button"
              title={t("photo.identity.delete")}
              onClick={() => onDelete(it.id)}
              className="opacity-0 group-hover:opacity-100 transition-opacity text-destructive"
            >
              <X className="h-3 w-3" />
            </button>
          </span>
        ))}

        {/* 新建身份 */}
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button size="sm" variant="outline" className="h-6 px-2 text-xs">
              <Plus className="h-3 w-3 mr-1" /> {t("photo.identity.new")}
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle className="flex items-center gap-1.5 text-base">
                <Fingerprint className="h-4 w-4" /> {t("photo.identity.new")}
              </DialogTitle>
            </DialogHeader>

            <div className="space-y-3">
              {/* 类型 */}
              <div className="flex gap-2">
                {(["product", "model"] as const).map((tp) => (
                  <Button key={tp} size="sm" variant={type === tp ? "default" : "outline"} onClick={() => setType(tp)}>
                    {tp === "product" ? t("photo.identity.type-product") : t("photo.identity.type-model")}
                  </Button>
                ))}
              </div>

              <div>
                <Label className="text-xs">{t("photo.identity.name")}</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} placeholder={t("photo.identity.name-ph")} className="mt-1" />
              </div>

              <div>
                <Label className="text-xs">{t("photo.identity.subject")}</Label>
                <Textarea value={subject} onChange={(e) => setSubject(e.target.value)} placeholder={t("photo.identity.subject-ph")} className="mt-1" rows={2} />
              </div>

              {/* 参考图（来自左侧已选） */}
              <div>
                <Label className="text-xs">{t("photo.identity.refs")}</Label>
                {selectedImageIds.length === 0 ? (
                  <p className="mt-1 text-xs text-destructive">{t("photo.identity.no-refs")}</p>
                ) : (
                  <div className="mt-1 flex flex-wrap gap-1">
                    {selectedImageIds.map((id) => {
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

      {selectedIdentityId && (
        <p className="mt-1 text-[10px] text-muted-foreground">{t("photo.identity.hint")}</p>
      )}
    </div>
  );
};

export default IdentityPanel;
