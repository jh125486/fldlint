import { init } from "@launchdarkly/node-server-sdk";
const client = init("sdk-key");
const v = await client.boolVariation("my-flag", ctx, false);
