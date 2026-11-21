import { init } from "@launchdarkly/node-server-sdk";
const client = init("sdk-key");
const v = await client.getBooleanValue("my-flag", false, { targetingKey: "u" });
