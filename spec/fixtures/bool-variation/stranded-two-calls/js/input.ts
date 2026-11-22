import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const client = OpenFeature.getClient();

export async function flags(userKey: string): Promise<[boolean, boolean]> {
  const ld = init("sdk-key");
  const checkout = await ld.boolVariation("new-checkout", { kind: "user", key: userKey }, false);
  const search = await ld.boolVariation("new-search", { kind: "user", key: userKey }, false);
  return [checkout, search];
}
