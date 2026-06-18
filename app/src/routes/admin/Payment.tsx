import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useTranslation } from "react-i18next";
import { useEffect, useState } from "react";
import { getPaymentOrders, recheckOrderStatus, PaymentOrder } from "@/payment/request";
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
import { Input } from "@/components/ui/input";
import { ChevronLeft, ChevronRight, Loader2, RefreshCw, Search } from "lucide-react";
import { toast } from "sonner";
import { useDebounce } from "use-debounce";

export default function Payment() {
  const { t } = useTranslation();
  const [data, setData] = useState<PaymentOrder[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [search, setSearch] = useState("");
  const [debouncedSearch] = useDebounce(search, 500);
  const [loading, setLoading] = useState(true);
  const pageSize = 20;

  const fetchData = async () => {
    setLoading(true);
    try {
      const res = await getPaymentOrders(page, debouncedSearch);
      if (res.status) {
        setData(res.data || []);
        setTotal(res.total || 0);
      }
    } catch {
      toast.error(t("admin.license.load-error"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [page, debouncedSearch]);

  const handleRecheck = async (orderId: string, service: string) => {
    const res = await recheckOrderStatus(orderId, service);
    if (res.status && res.is_changed) {
      toast.success(t("admin.license.pro-authorized"));
      fetchData();
    } else {
      toast.info(`Order state: ${res.order_state}`);
    }
  };

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className={`system`}>
      <Card className={`admin-card system-card`}>
        <CardHeader>
          <CardTitle>{t("admin.license.purchase")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-2 mb-4">
            <div className="relative flex-1 max-w-sm">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={t("record.query")}
                className="pl-9"
                value={search}
                onChange={(e) => { setSearch(e.target.value); setPage(0); }}
              />
            </div>
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
                    <TableHead>{t("record.type")}</TableHead>
                    <TableHead>Service</TableHead>
                    <TableHead>Amount</TableHead>
                    <TableHead>Order ID</TableHead>
                    <TableHead>State</TableHead>
                    <TableHead>{t("record.created-at")}</TableHead>
                    <TableHead>Actions</TableHead>
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
                    data.map((item) => (
                      <TableRow key={item.order_id}>
                        <TableCell>{item.username}</TableCell>
                        <TableCell><Badge variant="outline">{item.type}</Badge></TableCell>
                        <TableCell>{item.service}</TableCell>
                        <TableCell>{item.amount}</TableCell>
                        <TableCell className="max-w-[120px] truncate font-mono text-xs">{item.order_id}</TableCell>
                        <TableCell>
                          <Badge variant={item.state ? "default" : "secondary"}>
                            {item.state ? "Paid" : "Pending"}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-xs">{item.created_at}</TableCell>
                        <TableCell>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleRecheck(item.order_id, item.service)}
                          >
                            <RefreshCw className="w-3.5 h-3.5" />
                          </Button>
                        </TableCell>
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
