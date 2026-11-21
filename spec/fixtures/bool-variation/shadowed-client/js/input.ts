import { init } from "launchdarkly-node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function audited(userKey: string): Promise<boolean> {
  const client = createAuditLogger();
  const on = await ldClient.boolVariation("audit-log", { kind: "user", key: userKey }, false);
  client.log("audit-log", on);
  return on;
}

function createAuditLogger() {
  return { log: (name: string, value: boolean) => console.log(name, value) };
}
