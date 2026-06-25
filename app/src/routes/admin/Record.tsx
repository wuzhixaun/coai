import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useTranslation } from "react-i18next";
import { useEffect, useState } from "react";
import axios from "axios";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ChevronLeft, ChevronRight, Loader2 } from "lucide-react";
import { toast } from "sonner";

type RecordItem = {
  username: string;
  type: string;
  model: string;
  token_name: string;
  input_tokens: number;
  output_tokens: number;
  quota: number;
  duration: number;
  detail: string;
  channel_name: string;
  created_at: string;
};

type RecordStats = {
  billing_today: number;
  billing_month: number;
  request_today: number;
  request_month: number;
  rpm: number;
  tpm: number;
};

export default function Record() {
  const { t } = useTranslation();
  const [data, setData] = useState<RecordItem[]>([]);
  const [stats, setStats] = useState<RecordStats | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const pageSize = 20;

  const fetchData = async () => {
    setLoading(true);
    try {
      const [listRes, statsRes] = await Promise.all([
        axios.get(`/admin/record/list?page=${page}`),
        axios.post("/admin/record/stats"),
      ]);
      if (listRes.data) {
        setData(listRes.data.records || []);
        setTotal(listRes.data.total || 0);
      }
      if (statsRes.data) {
        setStats(statsRes.data);
      }
    } catch {
      toast.error(t("admin.license.load-error"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [page]);

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className={`system`}>
      <Card className={`admin-card system-card`}>
        <CardHeader>
          <CardTitle>{t("record.title")}</CardTitle>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="general">
            <TabsList className="mb-4">
              <TabsTrigger value="general">{t("record.image.general-tab")}</TabsTrigger>
              <TabsTrigger value="image">{t("record.image.tab")}</TabsTrigger>
            </TabsList>

            <TabsContent value="general">
              {stats && (
                <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3 mb-6">
                  <StatBox label={t("record.billing-today")} value={stats.billing_today.toFixed(2)} />
                  <StatBox label={t("record.billing-month")} value={stats.billing_month.toFixed(2)} />
                  <StatBox label={t("record.rpm-tips")} value={stats.request_today.toString()} />
                  <StatBox label={t("record.tpm-tips")} value={stats.request_month.toString()} />
                  <StatBox label="RPM" value={stats.rpm.toFixed(1)} />
                  <StatBox label="TPM" value={stats.tpm.toFixed(1)} />
                </div>
              )}

              {loading ? (
                <div className="flex justify-center py-12">
                  <Loader2 className="w-6 h-6 animate-spin" />
                </div>
              ) : (
                <>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("record.user")}</TableHead>
                        <TableHead>{t("record.type")}</TableHead>
                        <TableHead>{t("record.model")}</TableHead>
                        <TableHead>{t("record.token")}</TableHead>
                        <TableHead>{t("record.quota")}</TableHead>
                        <TableHead>{t("record.duration")}</TableHead>
                        <TableHead>{t("record.detail")}</TableHead>
                        <TableHead>{t("record.created-at")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {data.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={8} className="text-center text-muted-foreground py-8">
                            {t("record.query")}
                          </TableCell>
                        </TableRow>
                      ) : (
                        data.map((item, idx) => (
                          <TableRow key={idx}>
                            <TableCell>{item.username}</TableCell>
                            <TableCell>
                              <Badge variant="outline">{item.type}</Badge>
                            </TableCell>
                            <TableCell className="max-w-[120px] truncate">{item.model}</TableCell>
                            <TableCell className="max-w-[80px] truncate">{item.token_name}</TableCell>
                            <TableCell>{item.quota}</TableCell>
                            <TableCell>{item.duration}s</TableCell>
                            <TableCell className="max-w-[100px] truncate">{item.detail}</TableCell>
                            <TableCell className="text-xs">{item.created_at}</TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>

                  {totalPages > 1 && (
                    <div className="flex items-center justify-end gap-2 mt-4">
                      <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage(page - 1)}>
                        <ChevronLeft className="w-4 h-4" />
                      </Button>
                      <span className="text-sm">{page + 1} / {totalPages}</span>
                      <Button variant="outline" size="sm" disabled={page >= totalPages - 1} onClick={() => setPage(page + 1)}>
                        <ChevronRight className="w-4 h-4" />
                      </Button>
                    </div>
                  )}
                </>
              )}
            </TabsContent>

            <TabsContent value="image">
              <ImageUsage />
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </div>
  );
}

type ImageRecordItem = {
  username: string;
  source: string;
  model: string;
  channel_name: string;
  image_count: number;
  quota: number;
  duration: number;
  status: string;
  request_id: string;
  code: number;
  message: string;
  created_at: string;
};

type ImageModelStat = {
  model: string;
  image_count: number;
  quota: number;
};

type ImageStats = {
  images_today: number;
  images_month: number;
  billing_today: number;
  billing_month: number;
  success_today: number;
  failed_today: number;
  top_models: ImageModelStat[];
};

function ImageUsage() {
  const { t } = useTranslation();
  const [data, setData] = useState<ImageRecordItem[]>([]);
  const [stats, setStats] = useState<ImageStats | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [source, setSource] = useState("");
  const [status, setStatus] = useState("");
  const [loading, setLoading] = useState(true);
  const pageSize = 20;

  const fetchData = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ page: page.toString() });
      if (source) params.set("source", source);
      if (status) params.set("status", status);
      const [listRes, statsRes] = await Promise.all([
        axios.get(`/admin/image-record/list?${params.toString()}`),
        axios.post("/admin/image-record/stats"),
      ]);
      if (listRes.data) {
        setData(listRes.data.records || []);
        setTotal(listRes.data.total || 0);
      }
      if (statsRes.data) {
        setStats(statsRes.data);
      }
    } catch {
      toast.error(t("admin.license.load-error"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [page, source, status]);

  const totalPages = Math.ceil(total / pageSize);

  const setFilter = (next: string, current: string, setter: (v: string) => void) => {
    setter(next === current ? "" : next);
    setPage(0);
  };

  return (
    <>
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3 mb-6">
          <StatBox label={t("record.image.images-today")} value={stats.images_today.toString()} />
          <StatBox label={t("record.image.images-month")} value={stats.images_month.toString()} />
          <StatBox label={t("record.image.billing-today")} value={stats.billing_today.toFixed(2)} />
          <StatBox label={t("record.image.billing-month")} value={stats.billing_month.toFixed(2)} />
          <StatBox label={t("record.image.success-today")} value={stats.success_today.toString()} />
          <StatBox label={t("record.image.failed-today")} value={stats.failed_today.toString()} />
        </div>
      )}

      {stats && stats.top_models.length > 0 && (
        <div className="mb-6">
          <p className="text-sm text-muted-foreground mb-2">{t("record.image.top-models")}</p>
          <div className="flex flex-wrap gap-2">
            {stats.top_models.map((m) => (
              <Badge key={m.model} variant="secondary" className="font-normal">
                {m.model} · {m.image_count} · {m.quota.toFixed(2)}
              </Badge>
            ))}
          </div>
        </div>
      )}

      <div className="flex flex-wrap gap-2 mb-4">
        {["chat", "api", "photo"].map((s) => (
          <Button
            key={s}
            variant={source === s ? "default" : "outline"}
            size="sm"
            onClick={() => setFilter(s, source, setSource)}
          >
            {t(`record.image.sources.${s}`)}
          </Button>
        ))}
        <div className="w-px bg-border mx-1" />
        {["success", "failed"].map((s) => (
          <Button
            key={s}
            variant={status === s ? "default" : "outline"}
            size="sm"
            onClick={() => setFilter(s, status, setStatus)}
          >
            {t(`record.image.status-values.${s}`)}
          </Button>
        ))}
      </div>

      {loading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="w-6 h-6 animate-spin" />
        </div>
      ) : (
        <>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("record.user")}</TableHead>
                <TableHead>{t("record.image.source")}</TableHead>
                <TableHead>{t("record.model")}</TableHead>
                <TableHead>{t("record.image.count")}</TableHead>
                <TableHead>{t("record.quota")}</TableHead>
                <TableHead>{t("record.image.status")}</TableHead>
                <TableHead>{t("record.image.request-id")}</TableHead>
                <TableHead>{t("record.image.message")}</TableHead>
                <TableHead>{t("record.created-at")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={9} className="text-center text-muted-foreground py-8">
                    {t("record.image.empty")}
                  </TableCell>
                </TableRow>
              ) : (
                data.map((item, idx) => (
                  <TableRow key={idx}>
                    <TableCell>{item.username}</TableCell>
                    <TableCell>
                      <Badge variant="outline">
                        {t(`record.image.sources.${item.source}`, item.source)}
                      </Badge>
                    </TableCell>
                    <TableCell className="max-w-[120px] truncate">{item.model}</TableCell>
                    <TableCell>{item.image_count}</TableCell>
                    <TableCell>{item.quota}</TableCell>
                    <TableCell>
                      <Badge variant={item.status === "failed" ? "destructive" : "default"}>
                        {t(`record.image.status-values.${item.status}`, item.status)}
                      </Badge>
                    </TableCell>
                    <TableCell className="max-w-[120px] truncate text-xs font-mono">
                      {item.request_id}
                    </TableCell>
                    <TableCell className="max-w-[160px] truncate text-xs text-muted-foreground" title={item.message}>
                      {item.message}
                    </TableCell>
                    <TableCell className="text-xs">{item.created_at}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>

          {totalPages > 1 && (
            <div className="flex items-center justify-end gap-2 mt-4">
              <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage(page - 1)}>
                <ChevronLeft className="w-4 h-4" />
              </Button>
              <span className="text-sm">{page + 1} / {totalPages}</span>
              <Button variant="outline" size="sm" disabled={page >= totalPages - 1} onClick={() => setPage(page + 1)}>
                <ChevronRight className="w-4 h-4" />
              </Button>
            </div>
          )}
        </>
      )}
    </>
  );
}

function StatBox({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border bg-muted/30 p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="text-lg font-bold">{value}</p>
    </div>
  );
}
