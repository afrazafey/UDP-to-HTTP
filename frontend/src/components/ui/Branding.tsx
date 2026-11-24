const website = "https://afraaz.xyz";

export function DeveloperAttribution() {
  return (
    <div className="op-attribution">
      <span>Developed by Afraaz</span>
      <a href={website} target="_blank" rel="noopener noreferrer">afraaz.xyz</a>
    </div>
  );
}

export function EnvironmentIndicator({ value }: { value?: string }) {
  const environment = value === "Staging" || value === "Development"
    ? value
    : "Production";
  return (
    <span className={`op-environment${environment === "Production" ? " is-production" : ""}`}>
      <span aria-hidden="true" /> {environment}
    </span>
  );
}
