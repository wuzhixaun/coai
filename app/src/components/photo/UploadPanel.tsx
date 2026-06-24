import React, { useRef, useState } from "react";
import { Button } from "@/components/ui/button.tsx";
import { Upload, FolderOpen, Trash2 } from "lucide-react";
import type { PhotoImage } from "@/api/photo";

interface Props {
  images: PhotoImage[];
  selectedIds: string[];
  uploading: boolean;
  onUpload: (files: File[]) => void;
  onUploadFolder: (files: File[], folderName: string) => void;
  onToggleSelect: (id: string) => void;
  onSelectAll: () => void;
  onClearSelection: () => void;
  onRemove: (id: string) => void;
  onClearAll: () => void;
}

const ALLOWED = ["image/png", "image/jpeg", "image/webp", "image/bmp", "image/tiff"];

const UploadPanel: React.FC<Props> = ({
  images, selectedIds, uploading, onUpload, onUploadFolder,
  onToggleSelect, onSelectAll, onClearSelection, onRemove, onClearAll,
}) => {
  const fileRef = useRef<HTMLInputElement>(null);
  const folderRef = useRef<HTMLInputElement>(null);
  const [dragOver, setDragOver] = useState(false);

  const handleFiles = (files: FileList | null) => {
    if (!files) return;
    const valid = Array.from(files).filter(
      (f) => ALLOWED.includes(f.type) && f.size <= 50 * 1024 * 1024,
    );
    if (valid.length) onUpload(valid);
  };

  const handleFolder = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;
    const valid = Array.from(files).filter(
      (f) => ALLOWED.includes(f.type) && f.size <= 50 * 1024 * 1024,
    );
    if (valid.length) {
      const folderName = valid[0].webkitRelativePath?.split("/")[0] || "";
      onUploadFolder(valid, folderName);
    }
    e.target.value = "";
  };

  return (
    <div className="p-4 h-full flex flex-col">
      {/* Drop zone */}
      <div
        className={`border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors ${
          dragOver ? "border-primary bg-primary/5" : "border-gray-300"
        }`}
        onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
        onDragLeave={() => setDragOver(false)}
        onDrop={(e) => { e.preventDefault(); setDragOver(false); handleFiles(e.dataTransfer.files); }}
        onClick={() => fileRef.current?.click()}
      >
        <Upload className="mx-auto h-8 w-8 text-gray-400" />
        <p className="mt-2 text-sm text-gray-600">点击或拖拽图片到此处上传</p>
        <p className="text-xs text-gray-400">PNG / JPG / WebP / BMP，单文件最大 50MB</p>
        <input ref={fileRef} type="file" accept="image/*" multiple className="hidden"
          onChange={(e) => { handleFiles(e.target.files); e.target.value = ""; }} />
      </div>

      {/* Folder upload */}
      <input ref={folderRef} type="file" /* @ts-expect-error webkitdirectory */
        webkitdirectory="" multiple className="hidden" onChange={handleFolder} />
      <Button variant="outline" className="mt-2 w-full" onClick={() => folderRef.current?.click()}
        disabled={uploading}>
        <FolderOpen className="mr-2 h-4 w-4" /> 选择文件夹上传
      </Button>

      {/* Selection toolbar */}
      {images.length > 0 && (
        <>
          <div className="flex items-center gap-2 mt-3 text-sm">
            <Button size="sm" variant="ghost" onClick={onSelectAll}>全选</Button>
            <Button size="sm" variant="ghost" onClick={onClearSelection}>取消</Button>
            <Button size="sm" variant="ghost" className="text-red-500" onClick={onClearAll}><Trash2 className="h-3 w-3 mr-1" />清空</Button>
            <span className="ml-auto text-gray-500">已选 {selectedIds.length}/{images.length}</span>
          </div>

          {/* Thumbnail grid */}
          <div className="grid grid-cols-3 gap-2 mt-2 overflow-auto flex-1 content-start items-start auto-rows-min">
            {images.map((img) => (
              <div key={img.id} className={`relative rounded border-2 cursor-pointer ${
                selectedIds.includes(img.id) ? "border-primary" : "border-transparent"
              }`} onClick={() => onToggleSelect(img.id)}>
                <img src={img.url} alt={img.filename}
                  className="w-full h-20 object-cover rounded" />
                <p className="text-[10px] px-1 truncate text-gray-600">{img.filename}</p>
                {selectedIds.includes(img.id) && (
                  <span className="absolute top-1 right-1 bg-primary text-white rounded-full w-4 h-4 flex items-center justify-center text-[10px]">✓</span>
                )}
                <button className="absolute top-1 left-1 bg-red-500 text-white rounded-full w-4 h-4 flex items-center justify-center text-[10px]"
                  onClick={(e) => { e.stopPropagation(); onRemove(img.id); }}>×</button>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
};

export default UploadPanel;
