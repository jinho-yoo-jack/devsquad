import type { ButtonHTMLAttributes } from "react";
import { cn } from "@/lib/utils";

type Variant = "primary" | "secondary" | "ghost" | "danger" | "success";

const styles: Record<Variant, React.CSSProperties> = {
  primary: { background: "var(--status-running)", color: "#fff" },
  success: { background: "var(--status-done)", color: "#fff" },
  danger: { background: "var(--status-failed)", color: "#fff" },
  secondary: { background: "var(--surface-2)", color: "var(--text-1)", border: "1px solid var(--border)" },
  ghost: { background: "transparent", color: "var(--text-2)" },
};

export function Button({ variant = "secondary", loading, className, children, disabled, ...rest }:
  ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; loading?: boolean }) {
  return (
    <button
      {...rest}
      disabled={disabled || loading}
      style={styles[variant]}
      className={cn("inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-opacity disabled:opacity-50", className)}
    >
      {loading && <span className="inline-block h-3 w-3 animate-spin rounded-full border-2 border-current border-t-transparent" aria-hidden />}
      {children}
    </button>
  );
}
