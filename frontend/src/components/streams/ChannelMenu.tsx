import { Ellipsis, Radio } from "lucide-react";
import type { OperationsData, Stream } from "../../types/operations";
import { mutate } from "../../services/api";
import { playbackURL } from "../../utils/format";
import { Button, CopyButton } from "../ui";

export function ChannelIdentity({ stream }: { stream: Stream }) {
  return (
    <div className="op-channel-identity">
      <span className="op-channel-symbol" aria-hidden="true">
        <Radio size={16} />
      </span>
      <div>
        <a href={`#/streams/${stream.channel.id}`}>
          <strong>{stream.channel.name}</strong>
        </a>
        <small>
          {stream.channel.id}
          {stream.channel.group && ` · ${stream.channel.group}`}
        </small>
      </div>
    </div>
  );
}

export function ChannelMenu({
  stream,
  data,
  edit,
  run,
  busy,
  refresh,
}: {
  stream: Stream;
  data: OperationsData;
  edit: () => void;
  run: (task: () => Promise<unknown>, message: string) => Promise<boolean>;
  busy: boolean;
  refresh: () => void;
}) {
  const id = stream.channel.id;
  const active = ["starting", "waiting", "running", "retrying"].includes(
    stream.state,
  );
  const operations = stream.channel.enabled
    ? active
      ? ["stop", "restart"]
      : ["start"]
    : [];
  return (
    <details className="op-channel-menu">
      <summary aria-label={`Actions for ${stream.channel.name}`}>
        <Ellipsis size={18} />
      </summary>
      <div className="op-menu-items">
        <a href={`#/streams/${id}`}>View details</a>
        <CopyButton
          value={playbackURL(
            stream,
            data.settings.active,
            data.interfaces,
            String(data.settings.read_only.advertised_playback_host ?? ""),
          )}
          label="Copy playback URL"
        />
        <Button onClick={edit}>Edit channel</Button>
        {operations.map((operation) => (
          <Button
            key={operation}
            disabled={busy}
            onClick={async () => {
              if (
                await run(
                  () => mutate(`/api/v1/streams/${id}/${operation}`, "POST"),
                  "Receiver action accepted",
                )
              )
                refresh();
            }}
          >
            {operation[0].toUpperCase() + operation.slice(1)}
          </Button>
        ))}
      </div>
    </details>
  );
}
