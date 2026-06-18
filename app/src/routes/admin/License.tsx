import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card.tsx";
import { useTranslation } from "react-i18next";
import {
  CreditCard,
  Flame,
  InfoIcon,
  Loader2,
  Puzzle,
  QrCode,
  Sparkles,
} from "lucide-react";
import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge.tsx";
import { Button } from "@/components/ui/button.tsx";
import { toast } from "sonner";
import { useCurrency } from "@/store/info";
import axios from "axios";

type ModuleItemData = {
  id: string;
  price: number;
  bought: boolean;
};

type LicenseData = {
  domain: string;
  digest: string;
  modules: ModuleItemData[];
};

function ModuleItem({ id, price, bought }: ModuleItemData) {
  const { t } = useTranslation();
  const { symbol } = useCurrency();

  return (
    <div
      className={`rounded-md p-4 border flex flex-col bg-muted/20 transition hover:border-primary/20 cursor-pointer hover:-translate-y-1 duration-300`}
    >
      <p
        className={`flex flex-row items-center text-md font-bold text-primary mb-2.5`}
      >
        <Flame className={`w-4 h-4 mr-1`} />
        {t(`admin.license.modules.${id}.title`)}

        <Badge className={`ml-auto`} variant={`outline`}>
          {price === -1 ? (
            <p className={`text-2xs font-normal`}>
              {t("admin.license.modules.contact-for-price")}
            </p>
          ) : (
            <>
              <p className={`text-2xs font-normal`}>{symbol}</p>
              {price}
            </>
          )}
        </Badge>
      </p>
      <p className={`text-sm text-secondary`}>
        {t(`admin.license.modules.${id}.description`)}
      </p>
      <div className={`grow`} />
      <div className={`inline-flex flex-row mt-4`}>
        <div className={`grow`} />
        <Button
          variant={bought ? `outline` : `default`}
          onClick={() => {
            if (!bought) {
              if (price === -1) {
                window.open("https://www.coai.dev", "_blank");
              } else {
                toast.info(t("admin.license.modules.buy-tip"));
              }
            }
          }}
        >
          {t(
            bought
              ? "admin.license.modules.bought"
              : "admin.license.modules.not-bought",
          )}
        </Button>
      </div>
    </div>
  );
}

function License() {
  const { t } = useTranslation();
  const [data, setData] = useState<LicenseData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    axios
      .get("/admin/license")
      .then((res) => {
        setData(res.data);
        if (res.data.modules.some((m: ModuleItemData) => m.bought)) {
          toast.success(t("admin.license.pro-authorized"));
        } else {
          toast.info(t("admin.license.pro-required"));
        }
      })
      .catch(() => {
        setError(true);
        toast.error(t("admin.license.load-error"));
      })
      .finally(() => setLoading(false));
  }, [t]);

  if (loading) {
    return (
      <div className={`system flex items-center justify-center h-64`}>
        <Loader2 className={`w-8 h-8 animate-spin text-primary`} />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className={`system`}>
        <Card className={`admin-card system-card`}>
          <CardHeader>
            <CardTitle>{t("admin.license.title")}</CardTitle>
            <CardDescription>{t("admin.license.load-error")}</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  const hasPro = data.modules.some((m) => m.bought);
  const authorized = data.digest && data.digest !== "unauthorized" && hasPro;

  return (
    <div className={`system`}>
      <Card className={`admin-card system-card relative`}>
        <Sparkles
          className={`absolute bottom-4 right-4 select-none text-muted w-12 h-12 hover:text-gold/40 duration-500 transition cursor-pointer`}
        />
        <CardHeader>
          <CardTitle className={`flex w-full flex-row flex-wrap items-center`}>
            {t("admin.license.title")}

            <p
              className={`inline-flex flex-row items-center py-1 px-2 ml-auto text-xs border select-none cursor-pointer rounded-lg text-unread font-bold hover:border-muted-foreground transition duration-300`}
            >
              <QrCode className={`w-3.5 h-3.5 mr-1`} />
              0x{authorized ? data.digest.toUpperCase() : "UNAUTHORIZED"}
            </p>
          </CardTitle>
          <CardDescription>{t("admin.license.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <h2
            className={`mb-2 select-none font-bold text-lg inline-flex flex-row items-center`}
          >
            <InfoIcon className={`w-4 h-4 mr-1.5`} />
            {t("admin.license.info")}
          </h2>

          <div className={`mb-8`}>
            <table className={`w-fit h-fit`}>
              <tbody>
                <tr>
                  <td className={`font-bold`}>{t("admin.license.domain")}</td>
                  <td>
                    <Badge variant={`outline`} className={`m-1 ml-4`}>
                      {data.domain || "unknown"}
                    </Badge>
                  </td>
                </tr>
                <tr>
                  <td className={`font-bold`}>{t("admin.license.digest")}</td>
                  <td>
                    <Badge variant={`outline`} className={`m-1 ml-4`}>
                      0x
                      {authorized
                        ? data.digest.toUpperCase()
                        : "UNAUTHORIZED"}
                    </Badge>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          {!authorized && (
            <>
              <h2
                className={`mb-4 select-none font-bold text-lg inline-flex flex-row items-center`}
              >
                <CreditCard className={`w-4 h-4 mr-1.5`} />
                {t("admin.license.purchase")}
              </h2>
              <div
                className={`grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-8`}
              >
                <ModuleItem id={`coai-pro`} price={-1} bought={false} />
              </div>
            </>
          )}

          <h2
            className={`mb-4 select-none font-bold text-lg inline-flex flex-row items-center`}
          >
            <Puzzle className={`w-4 h-4 mr-1.5`} />
            {t("admin.license.module")}
          </h2>
          <div
            className={`grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-16`}
          >
            {data.modules
              .filter((m) => m.id !== "coai-pro")
              .map((mod) => (
                <ModuleItem
                  key={mod.id}
                  id={mod.id}
                  price={mod.price}
                  bought={mod.bought}
                />
              ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

export default License;
