import type { OperationsData, Stream } from "../../types/operations";
import { mutate, download } from "../../services/api";
import { playbackURL } from "../../utils/format";
import { Button, CopyButton, useAction } from "../ui";
export function StreamControls({
  stream,
  data,
  refresh,
  edit,
  remove,
  action,
}: {
  stream: Stream;
  data: OperationsData;
  refresh: () => void;
  edit: () => void;
  remove?: () => void;
  action: ReturnType<typeof useAction>;
}) {
  return (
    <div className="op-row-actions">
      {(
        [
          ["start", "Start"],
          ["stop", "Stop"],
          ["restart", "Restart"],
        ] as const
      ).map(([operation, label]) => (
        <Button
          key={operation}
          aria-label={`${label} ${stream.channel.name}`}
          disabled={
            action.busy || (!stream.channel.enabled && operation !== "stop")
          }
          onClick={async () => {
            if (
              await action.run(
                () =>
                  mutate(
                    `/api/v1/streams/${stream.channel.id}/${operation}`,
                    "POST",
                  ),
                `${label} confirmed; check receiver state for reception`,
              )
            )
              refresh();
          }}
        >
          {label}
        </Button>
      ))}
      <Button
        onClick={edit}
        disabled={action.busy}
        aria-label={`Edit ${stream.channel.name}`}
      >
        Edit
      </Button>
      {remove && (
        <Button
          onClick={remove}
          disabled={action.busy}
          aria-label={`Delete ${stream.channel.name}`}
        >
          Delete
        </Button>
      )}
      <CopyButton
        value={playbackURL(
          stream,
          data.settings.active,
          data.interfaces,
          String(data.settings.read_only.advertised_playback_host ?? ""),
        )}
      />
      <Button
        disabled={action.busy || !stream.channel.enabled}
        onClick={() =>
          void action.run(
            () =>
              download(
                `/api/v1/playlist.m3u?id=${encodeURIComponent(stream.channel.id)}`,
                `${stream.channel.id}.m3u`,
              ),
            "M3U entry downloaded",
          )
        }
      >
        M3U
      </Button>
      <a className="op-link" href={`#/streams/${stream.channel.id}`}>
        Details
      </a>
      <a className="op-link" href={`#/logs?stream_id=${stream.channel.id}`}>
        Logs
      </a>
    </div>
  );
}
