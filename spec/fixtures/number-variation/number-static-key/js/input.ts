import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function maxItems(userKey: string): Promise<number> {
  return await ldClient.numberVariation("max-items", { key: userKey }, 10);
}
