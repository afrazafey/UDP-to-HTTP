import { useState } from "react";
import type { Channel } from "../../Streaming";
import type { OperationsData, Stream } from "../../types/operations";
import { mutate } from "../../services/api";
import { Button, Dialog, ErrorBanner, Field, useAction } from "../ui";
export function ChannelDialog({
  data,
  stream,
  onClose,
  refresh,
}: {
  data: OperationsData;
  stream?: Stream;
  onClose: () => void;
  refresh: () => void;
}) {
  const settings = data.settings.active;
  const [manualID, setManualID] = useState(!!stream);
  const [form, setForm] = useState<Channel>(() =>
    stream
      ? { ...stream.channel }
      : {
          id: "",
          name: "",
          source_type: "multicast",
          source_address: "",
          source_port: 1234,
          interface:
            settings.network.multicast_interface ||
            data.interfaces.find((i) => i.up && i.multicast && i.ipv4.length)
              ?.name ||
            "",
          enabled: true,
          startup: settings.dashboard.default_startup,
          idle_timeout_seconds: settings.dashboard.default_idle_seconds,
          max_clients: settings.streaming.max_clients,
          client_buffer_bytes: settings.streaming.client_buffer_bytes,
          udp_receive_buffer: settings.streaming.udp_receive_buffer,
        },
  );
  const action = useAction();
  const [invalid, setInvalid] = useState("");
  const update = (value: Partial<Channel>) =>
    setForm((current) => ({ ...current, ...value }));
  const save = async () => {
    const octets = form.source_address.split(".");
    const ipValid =
      octets.length === 4 &&
      octets.every(
        (o) => /^\d{1,3}$/.test(o) && Number(o) >= 0 && Number(o) <= 255,
      );
    if (!ipValid) {
      setInvalid("Enter a literal IPv4 source address.");
      return;
    }
    if (
      form.source_type === "multicast" &&
      (Number(octets[0]) < 224 || Number(octets[0]) > 239)
    ) {
      setInvalid("A multicast group must be within 224.0.0.0/4.");
      return;
    }
    setInvalid("");
    if (
      await action.run(
        () =>
          mutate(
            stream ? `/api/v1/streams/${stream.channel.id}` : "/api/v1/streams",
            stream ? "PUT" : "POST",
            form,
          ),
        "Channel saved; inspect receiver status for reception",
      )
    ) {
      refresh();
      onClose();
    }
  };
  const choices = data.interfaces.filter(
    (i) =>
      i.up &&
      i.ipv4.length &&
      (form.source_type !== "multicast" || i.multicast),
  );
  return (
    <Dialog
      title={stream ? `Edit ${stream.channel.name}` : "Add stream"}
      onClose={() => !action.busy && onClose()}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault();
          void save();
        }}
      >
        {(action.error || invalid) && (
          <ErrorBanner>{invalid || action.error}</ErrorBanner>
        )}
        {stream && (
          <div className="op-callout">
            <strong>Receiver restart required for channel edits</strong>
            <p>
              Saving replaces the receiver and disconnects its {stream.clients}{" "}
              active viewer(s). Failed validation or persistence preserves the
              current channel configuration.
            </p>
          </div>
        )}
        <div className="op-form-grid">
          <Field label="Channel name">
            <input
              required
              maxLength={128}
              value={form.name}
              onChange={(event) => {
                const name = event.target.value;
                update({
                  name,
                  ...(!manualID && !stream
                    ? {
                        id: name
                          .toLowerCase()
                          .replace(/[^a-z0-9]+/g, "-")
                          .replace(/^-|-$/g, "")
                          .slice(0, 64),
                      }
                    : {}),
                });
              }}
            />
          </Field>
          <Field
            label="Channel ID"
            help="Stable playback URL identifier; read-only after creation."
          >
            <input
              required
              disabled={!!stream}
              pattern="[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}"
              maxLength={64}
              value={form.id}
              onChange={(event) => {
                setManualID(true);
                update({ id: event.target.value });
              }}
            />
          </Field>
          <Field label="Source type">
            <select
              value={form.source_type}
              onChange={(event) =>
                update({
                  source_type: event.target.value as Channel["source_type"],
                  interface: "",
                })
              }
            >
              <option value="multicast">Multicast group</option>
              <option value="unicast">Unicast sender</option>
            </select>
          </Field>
          <Field
            label="Source IP"
            help={
              form.source_type === "multicast"
                ? "Exact approved multicast group"
                : "Expected sender IPv4; UDP is bound on the selected local interface"
            }
          >
            <input
              required
              value={form.source_address}
              onChange={(event) =>
                update({ source_address: event.target.value })
              }
            />
          </Field>
          <Field label="UDP destination port">
            <input
              required
              type="number"
              min={1}
              max={65535}
              value={form.source_port}
              onChange={(event) =>
                update({ source_port: Number(event.target.value) })
              }
            />
          </Field>
          <Field label="Network interface">
            <select
              required
              value={form.interface}
              onChange={(event) => update({ interface: event.target.value })}
            >
              <option value="">Select a compatible interface</option>
              {choices.flatMap((i) => [
                <option key={i.name} value={i.name}>
                  {i.name} • {i.ipv4.join(", ")}
                </option>,
                ...i.ipv4.map((ip) => (
                  <option key={`${i.name}-${ip}`} value={ip}>
                    {ip} • {i.name}
                  </option>
                )),
              ])}
            </select>
          </Field>
          <Field label="Channel enabled">
            <select
              value={String(form.enabled)}
              onChange={(event) =>
                update({ enabled: event.target.value === "true" })
              }
            >
              <option value="true">Enabled</option>
              <option value="false">Disabled</option>
            </select>
          </Field>
          <Field label="Startup behavior">
            <select
              value={form.startup}
              onChange={(event) =>
                update({ startup: event.target.value as Channel["startup"] })
              }
            >
              <option value="on_demand">On demand</option>
              <option value="always">Always receive</option>
            </select>
          </Field>
          <Field
            label="Idle timeout (seconds)"
            help="0 releases an on-demand receiver after its final viewer disconnects."
          >
            <input
              required
              type="number"
              min={0}
              max={86400}
              value={form.idle_timeout_seconds}
              onChange={(event) =>
                update({ idle_timeout_seconds: Number(event.target.value) })
              }
            />
          </Field>
          <Field label="Maximum clients">
            <input
              required
              type="number"
              min={1}
              max={settings.streaming.max_clients}
              value={form.max_clients}
              onChange={(event) =>
                update({ max_clients: Number(event.target.value) })
              }
            />
          </Field>
          <Field label="Client buffer (bytes)">
            <input
              required
              type="number"
              min={564}
              max={8388608}
              value={form.client_buffer_bytes}
              onChange={(event) =>
                update({ client_buffer_bytes: Number(event.target.value) })
              }
            />
          </Field>
          <Field label="UDP receive buffer (bytes)">
            <input
              required
              type="number"
              min={65536}
              max={67108864}
              value={form.udp_receive_buffer}
              onChange={(event) =>
                update({ udp_receive_buffer: Number(event.target.value) })
              }
            />
          </Field>
        </div>
        <div className="op-prose">
          <p>
            Backend validation enforces current source allowlists, reserved
            multicast restrictions and actual interface capabilities. Server
            policy changes pending restart do not authorize sources yet.
          </p>
        </div>
        <div className="op-form-actions">
          <Button type="button" disabled={action.busy} onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" className="primary" disabled={action.busy}>
            {action.busy ? "Saving…" : "Save channel"}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
