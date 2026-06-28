import React from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button.tsx";
import { Wand2, BookMarked, X } from "lucide-react";
import type { WorkflowTemplate, WorkflowStep, PhotoRecipe } from "@/api/photo";

interface Props {
  templates: WorkflowTemplate[];
  recipes: PhotoRecipe[];
  selectedCount: number;
  loading: boolean;
  onRun: (templateKey: string) => void;
  onRunSteps: (steps: WorkflowStep[]) => void;
  onDeleteRecipe: (id: string) => void;
}

// 套件图标：与功能按钮一致用 emoji，按模板 key 映射，未知用默认。
const TEMPLATE_ICONS: Record<string, string> = {
  apparel_listing: "👔",
  product_main: "🖼️",
};
const templateIcon = (key: string) => TEMPLATE_ICONS[key] || "✨";

// 一键成套：选图后点模板，串行产出整套素材（白底→场景→营销 等）
const WorkflowBar: React.FC<Props> = ({ templates, recipes, selectedCount, loading, onRun, onRunSteps, onDeleteRecipe }) => {
  const { t } = useTranslation();
  if (templates.length === 0 && recipes.length === 0) return null;
  const disabled = loading || selectedCount === 0;

  return (
    <div className="space-y-3">
      {/* 一键成套（与下方功能分组同样式：小标题 + outline 按钮行） */}
      {templates.length > 0 && (
        <div>
          <p className="text-[11px] font-medium text-muted-foreground mb-1.5 flex items-center gap-1">
            <Wand2 className="h-3 w-3" /> {t("photo.workflow.title")}
          </p>
          <div className="flex flex-wrap gap-2">
            {templates.map((tpl) => (
              <Button
                key={tpl.key}
                size="sm"
                variant="outline"
                disabled={disabled}
                onClick={() => onRun(tpl.key)}
                title={tpl.steps.map((s) => t(`photo.features.${s.feature}`, s.feature)).join(" → ")}
              >
                {templateIcon(tpl.key)} {tpl.name}
                <span className="ml-1 text-[10px] text-muted-foreground">{tpl.steps.length} 步</span>
              </Button>
            ))}
          </div>
        </div>
      )}

      {/* 我的配方 */}
      {recipes.length > 0 && (
        <div>
          <p className="text-[11px] font-medium text-muted-foreground mb-1.5 flex items-center gap-1">
            <BookMarked className="h-3 w-3" /> {t("photo.recipe.title")}
          </p>
          <div className="flex flex-wrap gap-2">
            {recipes.map((r) => (
              <span key={r.id} className="group inline-flex items-center">
                <Button
                  size="sm"
                  variant="outline"
                  disabled={disabled}
                  onClick={() => onRunSteps(r.steps)}
                  title={r.steps.map((s) => t(`photo.features.${s.feature}`, s.feature)).join(" → ")}
                >
                  🔖 {r.name}
                  <span className="ml-1 text-[10px] text-muted-foreground">{r.steps.length} 步</span>
                </Button>
                <button type="button" onClick={() => onDeleteRecipe(r.id)} title={t("photo.recipe.delete", "删除")}
                  className="ml-0.5 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity hover:text-destructive">
                  <X className="h-3.5 w-3.5" />
                </button>
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default WorkflowBar;
