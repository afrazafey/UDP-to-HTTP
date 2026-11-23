import {
  Component,
  cloneElement,
  isValidElement,
  createContext,
  useContext,
  useEffect,
  useId,
  useRef,
  useState,
  type ReactNode,
  type ReactElement,
} from "react";
import { createPortal } from "react-dom";
import { CheckCircle2, Copy, Info, X } from "lucide-react";
export function Button({
  children,
  className = "",
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button type="button" {...props} className={`op-button ${className}`}>
      {children}
    </button>
  );
}
export function Panel({
  title,
  description,
  action,
  children,
  className = "",
}: {
  title?: string;
  description?: string;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={`op-panel ${className}`}>
      {title && (
        <div className="op-panel-head">
          <div>
            <h2>{title}</h2>
            {description && <p>{description}</p>}
          </div>
          {action}
        </div>
      )}
      {children}
    </section>
  );
}
export function Badge({ state }: { state: string }) {
  return (
    <span className={`op-badge op-state-${state}`}>
      {state.replaceAll("_", " ")}
    </span>
  );
}
export function Stat({
  label,
  value,
  detail,
}: {
  label: string;
  value: ReactNode;
  detail?: string;
}) {
  return (
    <div className="op-stat">
      <span>{label}</span>
      <strong>{value ?? "—"}</strong>
      {detail && <small>{detail}</small>}
    </div>
  );
}
export function Empty({
  title,
  children,
}: {
  title: string;
  children?: ReactNode;
}) {
  return (
    <div className="op-empty">
      <Info size={25} />
      <h3>{title}</h3>
      {children && <p>{children}</p>}
    </div>
  );
}
export function ErrorBanner({ children }: { children: ReactNode }) {
  return (
    <div role="alert" className="op-error">
      {children}
    </div>
  );
}
export function Skeleton() {
  return (
    <div
      className="op-skeleton"
      role="status"
      aria-label="Loading service data"
    >
      <div />
      <div />
      <div />
    </div>
  );
}
export function Field({
  label,
  help,
  children,
}: {
  label: string;
  help?: string;
  children: ReactNode;
}) {
  const id = useId();
  return (
    <div className="op-field">
      <label htmlFor={id}>{label}</label>
      {isValidElement(children)
        ? cloneElement(
            children as ReactElement<{
              id?: string;
              "aria-describedby"?: string;
            }>,
            { id, "aria-describedby": help ? `${id}-help` : undefined },
          )
        : children}
      {help && <small id={`${id}-help`}>{help}</small>}
    </div>
  );
}
export function Definition({ items }: { items: [string, ReactNode][] }) {
  return (
    <dl className="op-definition">
      {items.map(([label, value]) => (
        <div key={label}>
          <dt>{label}</dt>
          <dd>{value ?? "Unavailable"}</dd>
        </div>
      ))}
    </dl>
  );
}
export function Dialog({
  title,
  children,
  onClose,
  footer,
}: {
  title: string;
  children: ReactNode;
  onClose: () => void;
  footer?: ReactNode;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const id = useId();
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    const root = document.getElementById("ops-shell");
    const wasInert = root?.inert;
    if (root) root.inert = true;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const first = ref.current?.querySelector<HTMLElement>(
      "input,select,button,[tabindex='0']",
    );
    first?.focus();
    const key = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onCloseRef.current();
      }
      if (event.key === "Tab") {
        const nodes = Array.from(
          ref.current?.querySelectorAll<HTMLElement>(
            "button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled),a[href],[tabindex='0']",
          ) ?? [],
        );
        const first = nodes[0],
          last = nodes.at(-1);
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault();
          last?.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault();
          first?.focus();
        }
      }
    };
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("keydown", key);
      document.body.style.overflow = overflow;
      if (root) root.inert = wasInert ?? false;
      previous?.focus();
    };
  }, []);
  return createPortal(
    <div className="op-dialog-backdrop">
      <div
        className="op-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby={id}
        ref={ref}
      >
        <header>
          <h2 id={id}>{title}</h2>
          <button
            className="op-icon"
            aria-label="Close dialog"
            onClick={onClose}
          >
            <X size={19} />
          </button>
        </header>
        <div className="op-dialog-content">{children}</div>
        {footer && <footer>{footer}</footer>}
      </div>
    </div>,
    document.body,
  );
}
interface Notice {
  id: number;
  message: string;
  error: boolean;
}
const Notifications = createContext<(message: string, error?: boolean) => void>(
  () => {},
);
export function ToastProvider({ children }: { children: ReactNode }) {
  const [notices, setNotices] = useState<Notice[]>([]);
  const next = useRef(0);
  const timers = useRef(new Set<ReturnType<typeof setTimeout>>());
  useEffect(() => () => timers.current.forEach(clearTimeout), []);
  const notify = (message: string, error = false) => {
    const id = ++next.current;
    setNotices((items) => [...items.slice(-3), { id, message, error }]);
    const timer = setTimeout(() => {
      setNotices((items) => items.filter((item) => item.id !== id));
      timers.current.delete(timer);
    }, 6000);
    timers.current.add(timer);
  };
  return (
    <Notifications.Provider value={notify}>
      {children}
      <div className="op-toasts" aria-live="polite">
        {notices.map((item) => (
          <div
            key={item.id}
            className={item.error ? "error" : ""}
            role="status"
          >
            <CheckCircle2 size={18} />
            <span>{item.message}</span>
            <button
              aria-label="Dismiss notification"
              onClick={() =>
                setNotices((items) =>
                  items.filter((other) => other.id !== item.id),
                )
              }
            >
              <X size={16} />
            </button>
          </div>
        ))}
      </div>
    </Notifications.Provider>
  );
}
export const useToast = () => useContext(Notifications);
export function CopyButton({
  value,
  label = "Copy URL",
}: {
  value: string;
  label?: string;
}) {
  const notify = useToast();
  return (
    <Button
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(value);
          notify("Copied to clipboard");
        } catch {
          notify(`Clipboard unavailable. ${value}`, true);
        }
      }}
    >
      <Copy size={15} />
      {label}
    </Button>
  );
}
export function useAction() {
  const ref = useRef(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const notify = useToast();
  const run = async (action: () => Promise<unknown>, message: string) => {
    if (ref.current) return false;
    ref.current = true;
    setBusy(true);
    setError("");
    try {
      await action();
      notify(message);
      return true;
    } catch (failure) {
      const text =
        failure instanceof Error ? failure.message : "Operation failed";
      setError(text);
      notify(text, true);
      return false;
    } finally {
      ref.current = false;
      setBusy(false);
    }
  };
  return { run, busy, error };
}
export class ErrorBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    return this.state.failed ? (
      <div className="op-crash">
        <h1>The dashboard could not render</h1>
        <p>Reload to reconnect to the service.</p>
        <Button onClick={() => window.location.reload()}>
          Reload dashboard
        </Button>
      </div>
    ) : (
      this.props.children
    );
  }
}
