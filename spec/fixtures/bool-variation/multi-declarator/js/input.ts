import { init } from "launchdarkly-node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const retries = 3, ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function useFastPath(userKey: string): Promise<boolean> {
  console.log("retries", retries);
  return await ldClient.boolVariation("fast-path", { kind: "user", key: userKey }, true);
}
