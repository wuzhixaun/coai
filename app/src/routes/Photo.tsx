import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import UploadPanel from "@/components/photo/UploadPanel";
import FeaturePanel from "@/components/photo/FeaturePanel";
import IdentityPanel from "@/components/photo/IdentityPanel";
import TaskTable from "@/components/photo/TaskTable";
import { usePhotoTask } from "@/hooks/usePhotoTask";
import { cn } from "@/components/ui/lib/utils.ts";
import { Images, Sparkles, ListChecks } from "lucide-react";

type MobileTab = "upload" | "feature" | "task";

const Photo: React.FC = () => {
  const { t } = useTranslation();
  const {
    images, imagesLoading, selectedIds, tasks, loading, uploading, uploadProgress,
    upload, uploadFolder, toggleSelect, selectAll, clearSelection,
    removeImage, clearAll, process, retryAction, deleteAction,
    refreshTask, refreshAll,
    identities, selectedIdentityId, setSelectedIdentityId,
    createIdentityAction, deleteIdentityAction,
  } = usePhotoTask();

  const [mobileTab, setMobileTab] = useState<MobileTab>("upload");

  const activeCount = tasks.filter((t) => ["pending", "processing"].includes(t.status)).length;

  const tabs: { key: MobileTab; label: string; icon: React.ReactNode; badge?: number }[] = [
    { key: "upload", label: t("photo.tabs.images"), icon: <Images className="h-4 w-4" />, badge: images.length },
    { key: "feature", label: t("photo.tabs.feature"), icon: <Sparkles className="h-4 w-4" /> },
    { key: "task", label: t("photo.tabs.task"), icon: <ListChecks className="h-4 w-4" />, badge: activeCount },
  ];

  return (
    <div
      className="flex flex-col lg:flex-row bg-background text-foreground"
      style={{ height: "calc(100vh - 64px)" }}
    >
      {/* Mobile tab bar (hidden on desktop) */}
      <div className="lg:hidden flex border-b bg-card flex-shrink-0">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            className={cn(
              "flex-1 flex items-center justify-center gap-1.5 py-3 text-sm font-medium border-b-2 transition-colors",
              mobileTab === tab.key
                ? "border-primary text-primary"
                : "border-transparent text-muted-foreground",
            )}
            onClick={() => setMobileTab(tab.key)}
          >
            {tab.icon}
            {tab.label}
            {tab.badge ? (
              <span className="ml-0.5 rounded-full bg-primary/10 text-primary text-[10px] px-1.5 py-0.5 leading-none">
                {tab.badge}
              </span>
            ) : null}
          </button>
        ))}
      </div>

      {/* Left: Upload Panel */}
      <div
        className={cn(
          "overflow-auto bg-card",
          mobileTab === "upload" ? "flex-1" : "hidden",
          "lg:block lg:flex-none lg:w-80 lg:flex-shrink-0 lg:border-r",
        )}
      >
        <UploadPanel
          images={images} imagesLoading={imagesLoading} selectedIds={selectedIds} uploading={uploading} uploadProgress={uploadProgress}
          onUpload={upload} onUploadFolder={uploadFolder}
          onToggleSelect={toggleSelect} onSelectAll={selectAll}
          onClearSelection={clearSelection} onRemove={removeImage} onClearAll={clearAll}
        />
      </div>

      {/* Center: Feature Panel */}
      <div
        className={cn(
          "overflow-auto bg-card lg:border-r",
          mobileTab === "feature" ? "flex-1" : "hidden",
          "lg:block lg:flex-1",
        )}
      >
        <IdentityPanel
          identities={identities}
          selectedIdentityId={selectedIdentityId}
          selectedImageIds={selectedIds}
          images={images}
          onSelect={setSelectedIdentityId}
          onCreate={createIdentityAction}
          onDelete={deleteIdentityAction}
        />
        <FeaturePanel
          selectedCount={selectedIds.length} loading={loading}
          onProcess={(features, paramsMap, model) => process(features, paramsMap, model)}
        />
      </div>

      {/* Right: Task Table */}
      <div
        className={cn(
          "overflow-auto bg-muted/20",
          mobileTab === "task" ? "flex-1" : "hidden",
          "lg:block lg:flex-1",
        )}
      >
        <TaskTable
          tasks={tasks}
          images={images}
          onDelete={deleteAction}
          onRetry={retryAction}
          onRefreshTask={refreshTask}
          onRefreshAll={refreshAll}
        />
      </div>
    </div>
  );
};

export default Photo;
