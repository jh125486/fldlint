import { init, LDMigrationStage } from "@launchdarkly/node-server-sdk";
import { OpenFeature } from "@openfeature/server-sdk";

const ldClient = init("sdk-key");
const client = OpenFeature.getClient();

export async function dbStage(userKey: string): Promise<string> {
  const variation = await ldClient.migrationVariation("db-migration", { key: userKey }, LDMigrationStage.Off);
  return variation.value;
}
