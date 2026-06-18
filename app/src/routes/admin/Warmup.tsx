import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useTranslation } from "react-i18next";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import { Copy, Loader2, Play, ServerCrash } from "lucide-react";

export default function Warmup() {
  const { t } = useTranslation();
  const [urls, setUrls] = useState("");
  const [loading, setLoading] = useState(false);
  const [results, setResults] = useState<{ url: string; status: number; duration: number }[]>([]);

  const handleWarmup = async () => {
    const urlList = urls.split("\n").map((u) => u.trim()).filter(Boolean);
    if (urlList.length === 0) {
      toast.error("Please enter at least one URL");
      return;
    }

    setLoading(true);
    setResults([]);

    const newResults = [];
    for (const url of urlList) {
      const start = performance.now();
      try {
        const controller = new AbortController();
        setTimeout(() => controller.abort(), 10000);
        const resp = await fetch(url, { signal: controller.signal });
        newResults.push({ url, status: resp.status, duration: Math.round(performance.now() - start) });
      } catch {
        newResults.push({ url, status: 0, duration: Math.round(performance.now() - start) });
      }
    }
    setResults(newResults);
    setLoading(false);
    toast.success(t("admin.license.pro-authorized"));
  };

  const copyUrls = () => {
    const text = results.map((r) => r.url).join("\n");
    navigator.clipboard.writeText(text);
    toast.success("Copied");
  };

  return (
    <div className={`system`}>
      <Card className={`admin-card system-card`}>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ServerCrash className="w-5 h-5" />
            {t("admin.cdn.warmup")}
          </CardTitle>
          <CardDescription>{t("admin.cdn.warm-tip")}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="mb-4">
            <Textarea
              placeholder="https://example.com/api/v1/chat/completions&#10;https://example.com/api/v1/models"
              rows={6}
              value={urls}
              onChange={(e) => setUrls(e.target.value)}
              className="font-mono text-sm"
            />
          </div>

          <div className="flex gap-2 mb-6">
            <Button onClick={handleWarmup} disabled={loading || !urls.trim()}>
              {loading ? <Loader2 className="w-4 h-4 mr-1 animate-spin" /> : <Play className="w-4 h-4 mr-1" />}
              Start Warmup
            </Button>
            {results.length > 0 && (
              <Button variant="outline" onClick={copyUrls}>
                <Copy className="w-4 h-4 mr-1" />
                {t("admin.cdn.copy-data")}
              </Button>
            )}
          </div>

          {results.length > 0 && (
            <div className="space-y-2">
              {results.map((r, idx) => (
                <div key={idx} className="flex items-center gap-3 rounded-md border px-3 py-2 text-sm">
                  <Badge variant={r.status >= 200 && r.status < 400 ? "default" : "destructive"}>
                    {r.status || "ERR"}
                  </Badge>
                  <span className="flex-1 truncate font-mono text-xs">{r.url}</span>
                  <span className="text-muted-foreground text-xs">{r.duration}ms</span>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
