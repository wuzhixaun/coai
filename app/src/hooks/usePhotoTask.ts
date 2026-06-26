import { useState, useCallback, useRef, useEffect } from "react";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import type { PhotoImage, PhotoTask, PhotoIdentity, WorkflowTemplate, WorkflowStep, PhotoRecipe } from "@/api/photo";
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
  const [selectedBrandKitId, setSelectedBrandKitId] = useState<string>("");
  const [templates, setTemplates] = useState<WorkflowTemplate[]>([]);
  const [recipes, setRecipes] = useState<PhotoRecipe[]>([]);
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
    api.listWorkflowTemplates().then((data) => {
      if (Array.isArray(data)) setTemplates(data);
    }).catch(() => {});
    api.listRecipes().then((data) => {
      if (Array.isArray(data)) setRecipes(data);
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
  // 删除需同步后端，否则刷新后图库会从 DB 重新拉回（P0.1 后图库已持久化加载）。
  const removeImage = useCallback(async (id: string) => {
    setImages((prev) => prev.filter((x) => x.id !== id));
    setSelectedIds((prev) => prev.filter((x) => x !== id));
    try { await api.deleteImage(id); } catch (e) { console.error("Delete image failed:", e); }
  }, []);
  const clearAll = useCallback(async () => {
    const ids = images.map((i) => i.id);
    setImages([]);
    setSelectedIds([]);
    try { await Promise.allSettled(ids.map((id) => api.deleteImage(id))); }
    catch (e) { console.error("Clear images failed:", e); }
  }, [images]);

  const process = useCallback(async (features: string[], paramsMap: Record<string, Record<string, unknown>> = {}, model = "") => {
    if (selectedIds.length === 0) return;
    setLoading(true);
    for (const feature of features) {
      try {
        const params = paramsMap[feature] || {};
        const newTasks = await api.submitProcess(selectedIds, [feature], params, model, selectedIdentityId, selectedBrandKitId);
        if (Array.isArray(newTasks)) {
          newTasks.forEach((t) => {
            if (t.status !== "success" && t.status !== "failed") startPolling(t.task_id);
          });
          setTasks((prev) => [...newTasks, ...prev]);
        }
      } catch (e) { console.error("Process failed:", e); }
    }
    setLoading(false);
  }, [selectedIds, startPolling, selectedIdentityId, selectedBrandKitId]);

  // 一键成套：按模板或自定义步骤(配方)串行执行，结果聚合为一个 workflow 任务
  const submitWorkflowBody = useCallback(async (body: { template?: string; steps?: WorkflowStep[] }) => {
    if (selectedIds.length === 0) return;
    setLoading(true);
    try {
      const task = await api.submitWorkflow({
        ...body, image_ids: selectedIds,
        identity_id: selectedIdentityId, brand_kit_id: selectedBrandKitId,
      });
      if (task && task.task_id) {
        setTasks((prev) => [task, ...prev]);
        if (task.status !== "success" && task.status !== "failed") startPolling(task.task_id);
      }
    } catch (e) { console.error("Workflow failed:", e); }
    setLoading(false);
  }, [selectedIds, startPolling, selectedIdentityId, selectedBrandKitId]);

  const runWorkflow = useCallback((templateKey: string) => submitWorkflowBody({ template: templateKey }), [submitWorkflowBody]);
  const runWorkflowSteps = useCallback((steps: WorkflowStep[]) => submitWorkflowBody({ steps }), [submitWorkflowBody]);

  // ── 配方 ──
  const createRecipeAction = useCallback(async (name: string, steps: WorkflowStep[]) => {
    try {
      const created = await api.createRecipe({ name, steps });
      setRecipes((prev) => [created, ...prev]);
      toast.success(t("photo.recipe.saved", { name: created.name }));
    } catch (e) { console.error("Save recipe failed:", e); }
  }, [t]);

  const deleteRecipeAction = useCallback(async (id: string) => {
    try {
      await api.deleteRecipe(id);
      setRecipes((prev) => prev.filter((x) => x.id !== id));
    } catch (e) { console.error("Delete recipe failed:", e); }
  }, []);

  // ── 一致性身份 ──
  const refreshIdentities = useCallback(async () => {
    try {
      const data = await api.listIdentities();
      if (Array.isArray(data)) setIdentities(data);
    } catch { /* ignore */ }
  }, []);

  const createIdentityAction = useCallback(async (body: { type: string; name: string; ref_image_ids: string[]; subject_prompt?: string; color?: string }) => {
    const created = await api.createIdentity(body);
    setIdentities((prev) => [created, ...prev]);
    // 品牌资产与商品/模特身份是两条正交轴，分别记选中态
    if (created.type === "brandkit") setSelectedBrandKitId(created.id);
    else setSelectedIdentityId(created.id);
    toast.success(t("photo.identity.created", { name: created.name }));
    return created;
  }, [t]);

  const deleteIdentityAction = useCallback(async (id: string) => {
    try {
      await api.deleteIdentity(id);
      setIdentities((prev) => prev.filter((x) => x.id !== id));
      setSelectedIdentityId((cur) => (cur === id ? "" : cur));
      setSelectedBrandKitId((cur) => (cur === id ? "" : cur));
    } catch (e) { console.error("Delete identity failed:", e); }
  }, []);

  // 收藏：把单张图片一键收藏为可复用的商品身份（参考图资产化）
  const favoriteImage = useCallback(async (img: PhotoImage) => {
    try {
      await createIdentityAction({ type: "product", name: img.filename || "参考图", ref_image_ids: [img.id] });
    } catch (e) { console.error("Favorite image failed:", e); }
  }, [createIdentityAction]);

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
    selectedBrandKitId, setSelectedBrandKitId,
    refreshIdentities, createIdentityAction, deleteIdentityAction, favoriteImage,
    templates, runWorkflow, runWorkflowSteps,
    recipes, createRecipeAction, deleteRecipeAction };
}
