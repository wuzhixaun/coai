import { useState, useCallback, useRef, useEffect } from "react";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import type { PhotoImage, PhotoTask, PhotoIdentity } from "@/api/photo";
import * as api from "@/api/photo";

const POLL_INTERVAL = 10_000;

export function usePhotoTask() {
  const { t } = useTranslation();
  const [images, setImages] = useState<PhotoImage[]>([]);
  const [imagesLoading, setImagesLoading] = useState(true);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [tasks, setTasks] = useState<PhotoTask[]>([]);
  const [identities, setIdentities] = useState<PhotoIdentity[]>([]);
  const [selectedIdentityId, setSelectedIdentityId] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
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

  // Init: 并发拉取任务与图库，并对在途任务恢复轮询。
  // 此前只拉 tasks，刷新页面后图库（images）丢失，已上传图无法再选中处理。
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
    api.listImages().then((data) => {
      if (Array.isArray(data)) setImages(data);
    }).catch(() => {}).finally(() => setImagesLoading(false));
    api.listIdentities().then((data) => {
      if (Array.isArray(data)) setIdentities(data);
    }).catch(() => {});
    return () => { pollingRef.current.forEach((t) => clearInterval(t)); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const upload = useCallback(async (files: File[], folderName = "") => {
    setUploading(true);
    setUploadProgress(0);
    try {
      const data = await api.uploadImages(files, folderName, setUploadProgress);
      setImages((prev) => [...prev, ...data]);
      if (Array.isArray(data) && data.length > 0) {
        toast.success(t("photo.upload.success", { count: data.length }));
      }
      return data;
    } catch (e) {
      console.error("Upload failed:", e);
      toast.error(t("photo.upload.failed", { reason: e instanceof Error ? e.message : t("photo.upload.failed-retry") }));
    } finally {
      setUploading(false);
      setUploadProgress(0);
    }
  }, [t]);

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
        const newTasks = await api.submitProcess(selectedIds, [feature], params, model, selectedIdentityId);
        if (Array.isArray(newTasks)) {
          newTasks.forEach((t) => {
            if (t.status !== "success" && t.status !== "failed") startPolling(t.task_id);
          });
          setTasks((prev) => [...newTasks, ...prev]);
        }
      } catch (e) { console.error("Process failed:", e); }
    }
    setLoading(false);
  }, [selectedIds, startPolling, selectedIdentityId]);

  // ── 一致性身份 ──
  const refreshIdentities = useCallback(async () => {
    try {
      const data = await api.listIdentities();
      if (Array.isArray(data)) setIdentities(data);
    } catch { /* ignore */ }
  }, []);

  const createIdentityAction = useCallback(async (body: { type: string; name: string; ref_image_ids: string[]; subject_prompt?: string }) => {
    const created = await api.createIdentity(body);
    setIdentities((prev) => [created, ...prev]);
    setSelectedIdentityId(created.id);
    toast.success(t("photo.identity.created", { name: created.name }));
    return created;
  }, [t]);

  const deleteIdentityAction = useCallback(async (id: string) => {
    try {
      await api.deleteIdentity(id);
      setIdentities((prev) => prev.filter((x) => x.id !== id));
      setSelectedIdentityId((cur) => (cur === id ? "" : cur));
    } catch (e) { console.error("Delete identity failed:", e); }
  }, []);

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

  return { images, imagesLoading, selectedIds, tasks, loading, uploading, uploadProgress, upload, uploadFolder,
    toggleSelect, selectAll, clearSelection, removeImage, clearAll, process,
    retryAction, deleteAction, refreshTask, refreshAll,
    identities, selectedIdentityId, setSelectedIdentityId,
    refreshIdentities, createIdentityAction, deleteIdentityAction };
}
