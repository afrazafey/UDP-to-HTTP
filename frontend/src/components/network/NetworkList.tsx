import { useId } from "react";
import { X } from "lucide-react";

export type ListKind = "client" | "multicast" | "blocked" | "unicast" | "bind";
export const allMulticast = "224.0.0.0/4";

export function splitNetworkList(values: string[]) {
  return values.flatMap((value) => value.split(/[,\r\n]/)).map((value) => value.trim()).filter(Boolean);
}

function parseEntry(value: string, kind: ListKind, loopback: boolean) {
  const [address, length, extra] = value.split("/");
  if (extra !== undefined || (length !== undefined && !/^(0|[1-9]\d*)$/.test(length))) return null;
  const octets = address.split(".");
  if (octets.length === 4 && octets.every((octet) => /^(0|[1-9]\d{0,2})$/.test(octet) && Number(octet) <= 255)) {
    const prefix = length === undefined ? 32 : Number(length);
    if (prefix > 32) return null;
    const ip = octets.reduce((sum, octet) => sum * 256 + Number(octet), 0);
    const span = 2 ** (32 - prefix);
    const network = Math.floor(ip / span) * span;
    const first = network >>> 24;
    const multicast = first >= 224 && first <= 239;
    if ((kind === "multicast" || kind === "blocked") && (!multicast || prefix < 4)) return null;
    if (kind === "multicast" && prefix >= 24 && network >>> 8 === 224 * 65536) return null;
    if (kind === "bind" && (length !== undefined || multicast || ip === 0xffffffff)) return null;
    if (kind === "unicast" && (
      (prefix >= 4 && multicast) || (!loopback && prefix >= 8 && first === 127) ||
      (prefix === 32 && (ip === 0 || ip === 0xffffffff || (ip >>> 16) === 0xa9fe))
    )) return null;
    const canonical = [24, 16, 8, 0].map((shift) => (network >>> shift) & 255).join(".");
    return { key: `${canonical}/${prefix}`, value: length === undefined ? canonical : `${canonical}/${prefix}` };
  }
  if (kind !== "client" || !address.includes(":") || !/^[\da-f:.]+$/i.test(address)) return null;
  try {
    const canonical = new URL(`http://[${address}]/`).hostname.slice(1, -1);
    const prefix = length === undefined ? 128 : Number(length);
    if (prefix > 128) return null;
    if (canonical.startsWith("::ffff:")) {
      if (prefix < 96) return null;
      const ip = canonical.slice(7).split(":").flatMap((part) => {
        const word = Number.parseInt(part, 16);
        return [word >> 8, word & 255];
      }).join(".");
      return parseEntry(length === undefined ? ip : `${ip}/${prefix - 96}`, kind, loopback);
    }
    const [left, right] = canonical.split("::");
    const start = left ? left.split(":") : [];
    const end = right ? right.split(":") : [];
    const words = right === undefined ? start : [...start, ...Array<string>(8 - start.length - end.length).fill("0"), ...end];
    const masked = words.map((word, index) => {
      const remainder = Math.max(0, Math.min(16, prefix - index * 16));
      const span = 2 ** (16 - remainder);
      return (Math.floor(Number.parseInt(word, 16) / span) * span).toString(16);
    }).join(":");
    const network = new URL(`http://[${masked}]/`).hostname.slice(1, -1);
    return { key: `${network}/${prefix}`, value: length === undefined ? network : `${network}/${prefix}` };
  } catch {
    return null;
  }
}

export function analyzeNetworkList(values: string[], kind: ListKind, loopback = false) {
  const entries = new Map<string, string>();
  const invalid: string[] = [];
  let duplicates = 0;
  for (const value of splitNetworkList(values)) {
    const entry = parseEntry(value, kind, loopback);
    if (!entry) {
      invalid.push(value);
      continue;
    }
    if (entries.has(entry.key)) {
      duplicates++;
      if (entry.value.includes("/")) continue;
    }
    entries.set(entry.key, entry.value);
  }
  const valuesOnly = [...entries.values()];
  const constraint = kind === "bind" && valuesOnly.includes("0.0.0.0") && valuesOnly.length > 1
    ? "Use 0.0.0.0 by itself."
    : kind === "bind" && valuesOnly.length > 8 ? "Use at most eight bind addresses." : "";
  return { entries, invalid, duplicates, constraint };
}

export function NetworkList({
  label, help, example, kind, value, onChange, loopback = false,
}: {
  label: string;
  help: string;
  example: string;
  kind: ListKind;
  value: string[];
  onChange: (values: string[]) => void;
  loopback?: boolean;
}) {
  const id = useId();
  const { entries, invalid, duplicates, constraint } = analyzeNetworkList(value, kind, loopback);
  const hasError = invalid.length > 0 || Boolean(constraint);
  return (
    <div className="op-field op-network-list">
      <label htmlFor={id}>{label}</label>
      <span id={`${id}-help`}>{help} Separate entries with commas or line breaks.</span>
      <textarea id={id} rows={3} spellCheck={false} autoCapitalize="off" autoComplete="off"
        placeholder={example} value={value.join("\n")}
        aria-invalid={hasError} aria-describedby={`${id}-help ${id}-status`}
        onChange={(event) => onChange(event.target.value.split("\n"))}
        onBlur={() => {
          if (!hasError && duplicates === 0) onChange([...entries.values()]);
        }}
      />
      <div id={`${id}-status`} className="op-network-list-status" aria-live="polite">
        <span>{entries.size} {entries.size === 1 ? "entry" : "entries"}</span>
        {duplicates > 0 && <span>{duplicates} {duplicates === 1 ? "duplicate" : "duplicates"} will be merged on save</span>}
        {hasError && <span className="op-network-list-error">{constraint || `Invalid for this field: ${invalid.join(", ")}`}</span>}
      </div>
      {entries.size > 0 && (
        <ul className="op-network-chips" aria-label={`${label} entries`}>
          {[...entries].map(([key, entry]) => (
            <li key={key}>
              <code>{entry}</code>
              <button type="button" aria-label={`Remove ${entry}`} onClick={() => onChange(
                splitNetworkList(value).filter((raw) => parseEntry(raw, kind, loopback)?.key !== key),
              )}><X size={14} aria-hidden="true" /></button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
