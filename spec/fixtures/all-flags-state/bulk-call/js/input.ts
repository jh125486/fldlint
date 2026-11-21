import { init } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function snapshot(userKey: string) {
  const state = await ldClient.allFlagsState({ kind: "user", key: userKey });
  return state.toJSON();
}
