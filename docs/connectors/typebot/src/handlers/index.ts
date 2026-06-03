import { createRecordHandler } from "./createRecordHandler";
import { getRecordHandler } from "./getRecordHandler";
import { searchRecordsHandler } from "./searchRecordsHandler";
import { updateRecordHandler } from "./updateRecordHandler";
import { deleteRecordHandler } from "./deleteRecordHandler";
import {
  listTablesFetcherHandler,
  listWorkspacesFetcherHandler,
} from "./fetchersHandler";

export default [
  createRecordHandler,
  getRecordHandler,
  searchRecordsHandler,
  updateRecordHandler,
  deleteRecordHandler,
  listWorkspacesFetcherHandler,
  listTablesFetcherHandler,
];
