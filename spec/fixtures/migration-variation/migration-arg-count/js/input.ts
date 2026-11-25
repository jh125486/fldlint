import { init, LDMigrationStage } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function stages(userKey: string): Promise<void> {
  const ctx = { key: userKey };
  await ldClient.migrationVariation("short", ctx);
  await ldClient.migrationVariation("ok", ctx, LDMigrationStage.Off);
  await ldClient.migrationVariation("long", ctx, LDMigrationStage.Off, 1);
}
