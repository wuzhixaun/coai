import React from "react";
import UploadPanel from "@/components/photo/UploadPanel";
import FeaturePanel from "@/components/photo/FeaturePanel";
import TaskTable from "@/components/photo/TaskTable";
import { usePhotoTask } from "@/hooks/usePhotoTask";

const Photo: React.FC = () => {
  const {
    images, selectedIds, tasks, loading, uploading,
    upload, uploadFolder, toggleSelect, selectAll, clearSelection,
    removeImage, clearAll, process, retryAction, deleteAction,
    refreshTask, refreshAll,
  } = usePhotoTask();

  return (
    <div className="flex h-full" style={{ height: "calc(100vh - 64px)" }}>
      {/* Left: Upload Panel */}
      <div className="w-80 border-r bg-white overflow-auto flex-shrink-0">
        <UploadPanel
          images={images} selectedIds={selectedIds} uploading={uploading}
          onUpload={upload} onUploadFolder={uploadFolder}
          onToggleSelect={toggleSelect} onSelectAll={selectAll}
          onClearSelection={clearSelection} onRemove={removeImage} onClearAll={clearAll}
        />
      </div>

      {/* Center: Feature Panel */}
      <div className="flex-1 bg-white border-r overflow-auto">
        <FeaturePanel
          selectedCount={selectedIds.length} loading={loading}
          onProcess={(features, paramsMap, model) => process(features, paramsMap, model)}
        />
      </div>

      {/* Right: Task Table */}
      <div className="flex-1 bg-gray-50 overflow-auto">
        <TaskTable
          tasks={tasks}
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
