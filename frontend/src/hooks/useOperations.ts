import { useEffect, useState } from "react";
import { api } from "../services/api";
import type {
  Client,
  Dashboard,
  NetworkInterface,
  OperationsData,
  SettingsView,
  Stream,
  SystemInfo,
} from "../types/operations";
import { usePolling } from "./usePolling";
export function useOperations(paused: boolean) {
  const [interval, setInterval] = useState(5);
  const polling = usePolling<OperationsData>(
    async (signal) => {
      const [dashboard, streams, clients, interfaces, settings, system] =
        await Promise.all([
          api<Dashboard>("/api/v1/dashboard", {}, signal),
          api<{ streams: Stream[] }>("/api/v1/streams", {}, signal),
          api<{ clients: Client[] }>("/api/v1/clients", {}, signal),
          api<{ interfaces: NetworkInterface[] }>(
            "/api/v1/interfaces",
            {},
            signal,
          ),
          api<SettingsView>("/api/v1/settings", {}, signal),
          api<SystemInfo>("/api/v1/system", {}, signal),
        ]);
      return {
        dashboard,
        streams: streams.streams,
        clients: clients.clients,
        interfaces: interfaces.interfaces,
        settings,
        system,
      };
    },
    "operations",
    interval,
    paused,
  );
  useEffect(() => {
    if (polling.data)
      setInterval(polling.data.settings.active.dashboard.refresh_seconds);
  }, [polling.data]);
  return { ...polling, interval };
}
