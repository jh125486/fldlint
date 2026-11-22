import { init, LDClient } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ld: LDClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function showSearch(userKey: string): Promise<boolean> {
  return await ld.boolVariation("new-search", { kind: "user", key: userKey }, false);
}
