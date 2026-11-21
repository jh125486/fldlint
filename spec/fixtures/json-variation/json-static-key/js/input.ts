import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function uiConfig(userKey: string) {
  return await ldClient.jsonVariation("ui-config", { kind: "user", key: userKey }, { theme: "light" });
}
