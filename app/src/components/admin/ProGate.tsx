import { useEffect, useState } from "react";
import { Loader2 } from "lucide-react";
import axios from "axios";
import License from "@/routes/admin/License";

type ProGateProps = {
  children: React.ReactNode;
};

export default function ProGate({ children }: ProGateProps) {
  const [authorized, setAuthorized] = useState<boolean | null>(null);

  useEffect(() => {
    axios
      .get("/admin/license")
      .then((res) => {
        const modules = res.data?.modules || [];
        const hasPro = modules.some((m: { bought: boolean }) => m.bought);
        setAuthorized(hasPro);
      })
      .catch(() => setAuthorized(false));
  }, []);

  if (authorized === null) {
    return (
      <div className="system flex items-center justify-center h-64">
        <Loader2 className="w-8 h-8 animate-spin text-primary" />
      </div>
    );
  }

  if (!authorized) {
    return <License />;
  }

  return <>{children}</>;
}
