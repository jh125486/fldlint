import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function generic(userKey: string, rest: any[]): Promise<void> {
  const ctx = { key: userKey };
  await ldClient.variation("short", ctx);
  await ldClient.variation("plain", ctx, false);
  await ldClient.variation("callback", ctx, false, (err, res) => undefined);
  await ldClient.variation("long", ctx, false, (err, res) => undefined, 1);
  await ldClient.variation("spread", ctx, false, ...rest);
}
