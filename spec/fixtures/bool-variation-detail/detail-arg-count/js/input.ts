import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function details(userKey: string, rest: any[]): Promise<void> {
  const ctx = { kind: "user", key: userKey };
  await ldClient.boolVariationDetail("short", ctx);
  await ldClient.boolVariationDetail("ok", ctx, false);
  await ldClient.boolVariationDetail("long", ctx, false, () => undefined);
  await ldClient.boolVariationDetail("spread", ctx, ...rest);
}
