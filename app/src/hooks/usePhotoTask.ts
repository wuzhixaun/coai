import { useState, useCallback, useRef, useEffect } from "react";
import type { PhotoImage, PhotoTask } from "@/api/photo";
import * as api from "@/api/photo";

const POLL_INTERVAL = 10_000;

export function usePhotoTask() {
  const [images, setImages] = useState<PhotoImage[]>([]);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [tasks, setTasks] = useState<PhotoTask[]>([]);
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const pollingRef = useRef<Map<string, ReturnType<typeof setInterval>>>(new Map());
  const initialized = useRef(false);

  const stopPolling = useCallback((taskId: string) => {
    const timer = pollingRef.current.get(taskId);
    if (timer) { clearInterval(timer); pollingRef.current.delete(taskId); }
  }, []);

  const startPolling = useCallback((taskId: string) => {
    if (pollingRef.current.has(taskId)) return;
    const poll = async () => {
      try {
        const task = await api.getTask(taskId);
        setTasks((prev) => {
          const idx = prev.findIndex((t) => t.task_id === taskId);
          if (idx >= 0) { const n = [...prev]; n[idx] = task; return n; }
          return [task, ...prev];
        });
        if (task.status === "success" || task.status === "failed") stopPolling(taskId);
      } catch { stopPolling(taskId); }
    };
    poll();
    pollingRef.current.set(taskId, setInterval(poll, POLL_INTERVAL));
  }, [stopPolling]);

  // Init: load tasks once, restore polling for active ones
  useEffect(() => {
    if (initialized.current) return;
    initialized.current = true;
    api.listTasks().then((data) => {
      if (!Array.isArray(data)) return;
      setTasks(data);
      data.forEach((t) => {
        if (t.status === "pending" || t.status === "processing") startPolling(t.task_id);
      });
    }).catch(() => {});
    return () => { pollingRef.current.forEach((t) => clearInterval(t)); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const upload = useCallback(async (files: File[], folderName = "") => {
    setUploading(true);
    try {
      const data = await api.uploadImages(files, folderName);
      setImages((prev) => [...prev, ...data]);
      return data;
    } catch (e) { console.error("Upload failed:", e); }
    finally { setUploading(false); }
  }, []);

  const uploadFolder = useCallback(async (files: File[], folderName: string) => {
    return upload(files, folderName);
  }, [upload]);

  const toggleSelect = useCallback((id: string) => {
    setSelectedIds((prev) => prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]);
  }, []);

  const selectAll = useCallback(() => setSelectedIds(images.map((i) => i.id)), [images]);
  const clearSelection = useCallback(() => setSelectedIds([]), []);
  const removeImage = useCallback((id: string) => {
    setImages((prev) => prev.filter((x) => x.id !== id));
    setSelectedIds((prev) => prev.filter((x) => x !== id));
  }, []);
  const clearAll = useCallback(() => { setImages([]); setSelectedIds([]); }, []);

  const process = useCallback(async (features: string[], paramsMap: Record<string, Record<string, unknown>> = {}, model = "") => {
    if (selectedIds.length === 0) return;
    setLoading(true);
    for (const feature of features) {
      try {
        const params = paramsMap[feature] || {};
        const newTasks = await api.submitProcess(selectedIds, [feature], params, model);
        if (Array.isArray(newTasks)) {
          newTasks.forEach((t) => {
            if (t.status !== "success" && t.status !== "failed") startPolling(t.task_id);
          });
          setTasks((prev) => [...newTasks, ...prev]);
        }
      } catch (e) { console.error("Process failed:", e); }
    }
    setLoading(false);
  }, [selectedIds, startPolling]);

  const retryAction = useCallback(async (taskId: string) => {
    try {
      const task = await api.retryTask(taskId);
      setTasks((prev) => { const idx = prev.findIndex((t) => t.task_id === taskId); if (idx >= 0) { const n = [...prev]; n[idx] = task; return n; } return [task, ...prev]; });
      if (task.status !== "success" && task.status !== "failed") startPolling(taskId);
    } catch (e) { console.error("Retry failed:", e); }
  }, [startPolling]);

  const deleteAction = useCallback(async (taskId: string) => {
    try {
      await api.deleteTask(taskId);
      setTasks((prev) => prev.filter((t) => t.task_id !== taskId));
      stopPolling(taskId);
    } catch (e) { console.error("Delete failed:", e); }
  }, [stopPolling]);

  const refreshTask = useCallback(async (taskId: string) => {
    try {
      const task = await api.getTask(taskId);
      setTasks((prev) => { const idx = prev.findIndex((t) => t.task_id === taskId); if (idx >= 0) { const n = [...prev]; n[idx] = task; return n; } return prev; });
    } catch (e) { /* ignore */ }
  }, []);

  const refreshAll = useCallback(async () => {
    try {
      const data = await api.listTasks();
      if (Array.isArray(data)) setTasks(data);
    } catch (e) { /* ignore */ }
  }, []);

  return { images, selectedIds, tasks, loading, uploading, upload, uploadFolder,
    toggleSelect, selectAll, clearSelection, removeImage, clearAll, process,
    retryAction, deleteAction, refreshTask, refreshAll };
}
