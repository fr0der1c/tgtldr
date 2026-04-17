export type TimezoneOption = {
  value: string;
  label: string;
};

const fallbackZones = [
  "UTC",
  "Africa/Johannesburg",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/New_York",
  "Asia/Bangkok",
  "Asia/Hong_Kong",
  "Asia/Shanghai",
  "Asia/Singapore",
  "Asia/Tokyo",
  "Australia/Sydney",
  "Europe/Berlin",
  "Europe/London",
  "Europe/Paris"
];

export function listTimezoneOptions() {
  const zones = supportedTimezones();
  return zones.map((value) => ({
    value,
    label: `${cityName(value)}（${value}）`
  }));
}

function supportedTimezones() {
  try {
    const values = Intl.supportedValuesOf?.("timeZone") ?? fallbackZones;
    const unique = new Set(values);
    unique.add("UTC");
    return Array.from(unique).sort((left, right) => left.localeCompare(right));
  } catch {
    return [...fallbackZones].sort((left, right) => left.localeCompare(right));
  }
}

function cityName(timezone: string) {
  if (timezone === "UTC") {
    return "协调世界时";
  }

  const parts = timezone.split("/");
  const city = parts[parts.length - 1] ?? timezone;
  return city.replaceAll("_", " ");
}
