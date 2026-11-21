import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function maxItems(userKey: string): Promise<number> {
  return await client.getNumberValue("max-items", 10, { targetingKey: userKey });
}
