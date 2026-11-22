import { init } from "@launchdarkly/node-server-sdk";
const client = init("sdk-key");
const v = await client.boolVariation(name, ctx, false);
