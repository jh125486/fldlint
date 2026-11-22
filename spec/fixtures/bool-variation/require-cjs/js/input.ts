const { init } = require("@launchdarkly/node-server-sdk");
const { OpenFeature } = require("@openfeature/server-sdk");

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function checkoutEnabled(userKey: string): Promise<boolean> {
  return await ldClient.boolVariation("new-checkout", { kind: "user", key: userKey }, false);
}
