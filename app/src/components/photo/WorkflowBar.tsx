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

// 一键成套：选图后点模板，串行产出整套素材（白底→场景→营销 等）
const WorkflowBar: React.FC<Props> = ({ templates, recipes, selectedCount, loading, onRun, onRunSteps, onDeleteRecipe }) => {
  const { t } = useTranslation();
  if (templates.length === 0 && recipes.length === 0) return null;
  const disabled = loading || selectedCount === 0;

  return (
    <div className="border-b bg-card px-4 py-2 space-y-1.5">
      <div className="flex items-center gap-2 flex-wrap">
        <span className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
          <Wand2 className="h-3.5 w-3.5" /> {t("photo.workflow.title")}
        </span>
        {templates.map((tpl) => (
          <Button
            key={tpl.key}
            size="sm"
            variant="outline"
            className="h-7 text-xs"
            disabled={disabled}
            onClick={() => onRun(tpl.key)}
            title={tpl.steps.map((s) => t(`photo.features.${s.feature}`, s.feature)).join(" → ")}
          >
            {tpl.name}
            <span className="ml-1 text-[10px] text-muted-foreground">{tpl.steps.length} 步</span>
          </Button>
        ))}
      </div>

      {/* 我的配方 */}
      {recipes.length > 0 && (
        <div className="flex items-center gap-2 flex-wrap">
          <span className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
            <BookMarked className="h-3.5 w-3.5" /> {t("photo.recipe.title")}
          </span>
          {recipes.map((r) => (
            <span key={r.id}
              className="group flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs hover:bg-muted/40">
              <button type="button" disabled={disabled} onClick={() => onRunSteps(r.steps)}
                title={r.steps.map((s) => t(`photo.features.${s.feature}`, s.feature)).join(" → ")}
                className="flex items-center gap-1 disabled:opacity-50">
                {r.name}
                <span className="text-[10px] text-muted-foreground">{r.steps.length} 步</span>
              </button>
              <button type="button" onClick={() => onDeleteRecipe(r.id)}
                className="opacity-0 group-hover:opacity-100 transition-opacity text-destructive">
                <X className="h-3 w-3" />
              </button>
            </span>
          ))}
        </div>
      )}

      <p className="text-[10px] text-muted-foreground">{t("photo.workflow.hint")}</p>
    </div>
  );
};

export default WorkflowBar;
