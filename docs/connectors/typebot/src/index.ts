import { createBlock } from "@typebot.io/forge";
import { auth } from "./auth";
import { HorneroDbLogo } from "./logo";
import { createRecord } from "./actions/createRecord";
import { getRecord } from "./actions/getRecord";
import { searchRecords } from "./actions/searchRecords";
import { updateRecord } from "./actions/updateRecord";
import { deleteRecord } from "./actions/deleteRecord";

export const horneroDbBlock = createBlock({
  id: "hornerodb",
  name: "HorneroDB",
  tags: ["database"],
  LightLogo: HorneroDbLogo,
  DarkLogo: HorneroDbLogo,
  auth,
  actions: [createRecord, getRecord, searchRecords, updateRecord, deleteRecord],
  docsUrl: "https://docs.hornerodb.com/integrations/typebot",
});
