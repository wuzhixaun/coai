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
        </CardContent>
      </Card>
    </div>
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
