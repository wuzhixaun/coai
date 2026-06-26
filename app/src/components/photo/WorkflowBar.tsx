import React from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button.tsx";
import { Wand2 } from "lucide-react";
import type { WorkflowTemplate } from "@/api/photo";

interface Props {
  templates: WorkflowTemplate[];
  selectedCount: number;
  loading: boolean;
  onRun: (templateKey: string) => void;
}

// 一键成套：选图后点模板，串行产出整套素材（白底→场景→营销 等）
const WorkflowBar: React.FC<Props> = ({ templates, selectedCount, loading, onRun }) => {
  const { t } = useTranslation();
  if (templates.length === 0) return null;

  return (
    <div className="border-b bg-card px-4 py-2">
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
            disabled={loading || selectedCount === 0}
            onClick={() => onRun(tpl.key)}
            title={tpl.steps.map((s) => t(`photo.features.${s.feature}`, s.feature)).join(" → ")}
          >
            {tpl.name}
            <span className="ml-1 text-[10px] text-muted-foreground">{tpl.steps.length} 步</span>
          </Button>
        ))}
      </div>
      <p className="mt-1 text-[10px] text-muted-foreground">{t("photo.workflow.hint")}</p>
    </div>
  );
};

export default WorkflowBar;
