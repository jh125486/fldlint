import { init as ldInit } from "launchdarkly-node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = ldInit("sdk-key");
const client = OpenFeature.getClient();

export async function darkMode(userKey: string): Promise<boolean> {
  return await client.getBooleanValue("dark-mode", false, { kind: "user", key: userKey });
}
