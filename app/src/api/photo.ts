import axios from "axios";

export interface PhotoImage {
  id: string; filename: string; size: number; width: number; height: number;
  url: string; folder_name: string; created_at: string;
}

export interface PhotoTask {
  task_id: string; feature: string; status: "pending" | "processing" | "success" | "failed";
  image_ids: string[]; result_urls: string[]; error_message: string; progress: number;
  created_at: string; folder_name: string; total_images: number; processed_images: number;
  total_videos: number; processed_videos: number; completed_at?: string;
  source_filenames: string[]; submit_ids: string[];
}

export interface PhotoIdentity {
  id: string;
  type: "product" | "model" | "brandkit";
  name: string;
  ref_image_ids: string[];
  ref_image_urls: string[];
  seed: number;
  subject_prompt: string;
  color: string;
  created_at: string;
}

export interface FeatureConfig {
  channel_type: string; model?: string; system_prompt: string;
  templates?: { label: string; prompt: string }[];
  colors?: { value: string; label: string }[];
  languages?: { value: string; label: string }[];
  selling_points?: { value: string; label: string }[];
  sizes?: { value: string; label: string }[];
  categories?: { value: string; label: string }[];
}

export interface PromptsConfig {
  features: Record<string, FeatureConfig>;
  defaults: Record<string, string>;
}

export async function uploadImages(
  files: File[],
  folderName = "",
  onProgress?: (percent: number) => void,
): Promise<PhotoImage[]> {
  const fd = new FormData();
  files.forEach((f) => fd.append("files", f));
  if (folderName) fd.append("folder_name", folderName);
  const token = axios.defaults.headers.common["Authorization"];

  // 用 XHR 取代 fetch，以获得上传进度事件（fetch 不支持 upload progress）。
  // 保持与原实现完全一致的字面量 URL 与鉴权头，不引入 axios baseURL 以免改变路径解析。
  return new Promise<PhotoImage[]>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `/api/photo/upload`);
    if (token) xhr.setRequestHeader("Authorization", String(token));
    xhr.upload.onprogress = (e) => {
      if (onProgress && e.lengthComputable) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText) as PhotoImage[]);
        } catch {
          reject(new Error("upload response parse error"));
        }
      } else {
        reject(new Error(`Upload failed: ${xhr.status}`));
      }
    };
    xhr.onerror = () => reject(new Error("upload network error"));
    xhr.send(fd);
  });
}

export async function listImages(): Promise<PhotoImage[]> {
  const { data } = await axios.get("/photo/images");
  return data;
}

export async function deleteImage(id: string): Promise<void> {
  await axios.delete(`/photo/images/${id}`);
}

export async function submitProcess(
  imageIds: string[], features: string[],
  params: Record<string, unknown> = {}, channelOverride = "", identityId = "", brandKitId = "",
): Promise<PhotoTask[]> {
  const { data } = await axios.post("/photo/process", {
    image_ids: imageIds, features, params, channel_override: channelOverride,
    identity_id: identityId, brand_kit_id: brandKitId,
  });
  return data;
}

// ── 一致性身份（商品/模特）──────────────────────────────────
export async function listIdentities(type?: string): Promise<PhotoIdentity[]> {
  const { data } = await axios.get("/photo/identity", { params: type ? { type } : {} });
  return data;
}

export async function createIdentity(body: {
  type: string; name: string; ref_image_ids: string[]; subject_prompt?: string; color?: string;
}): Promise<PhotoIdentity> {
  const { data } = await axios.post("/photo/identity", body);
  return data;
}

export async function deleteIdentity(id: string): Promise<void> {
  await axios.delete(`/photo/identity/${id}`);
}

// ── 一键成套素材工作流 ──────────────────────────────────────
export interface WorkflowStep {
  feature: string;
  params?: Record<string, unknown>;
}

export interface WorkflowTemplate {
  key: string;
  name: string;
  steps: WorkflowStep[];
}

export async function listWorkflowTemplates(): Promise<WorkflowTemplate[]> {
  const { data } = await axios.get("/photo/workflow/templates");
  return data;
}

export async function submitWorkflow(body: {
  template?: string; steps?: WorkflowStep[]; image_ids: string[];
  identity_id?: string; brand_kit_id?: string;
}): Promise<PhotoTask> {
  const { data } = await axios.post("/photo/workflow", body);
  return data;
}

export async function listTasks(): Promise<PhotoTask[]> {
  const { data } = await axios.get("/photo/tasks");
  return data;
}

export async function getTask(taskId: string): Promise<PhotoTask> {
  const { data } = await axios.get(`/photo/tasks/${taskId}`);
  return data;
}

export async function deleteTask(taskId: string): Promise<void> {
  await axios.delete(`/photo/tasks/${taskId}`);
}

export async function retryTask(taskId: string): Promise<PhotoTask> {
  const { data } = await axios.post(`/photo/tasks/${taskId}/retry`);
  return data;
}

export async function getPrompts(): Promise<PromptsConfig> {
  const { data } = await axios.get("/photo/prompts");
  return data;
}

export function getDownloadFileUrl(url: string): string {
  return `/photo/download/file?url=${encodeURIComponent(url)}`;
}

export function getDownloadZipUrl(urls: string[]): string {
  return `/photo/download/zip?urls=${encodeURIComponent(urls.join(","))}`;
}
