import { isNotDefined } from "@typebot.io/lib/utils";

type ResponseMapping = {
  fieldName?: string;
  variableId?: string;
}[];

type VariableSetter = {
  set: (
    value:
      | { id: string; value: unknown }
      | { id: string; value: unknown }[],
  ) => void;
};

type Logger = {
  add: (message: string) => void;
};

// Walks a Response Mapping and assigns each entry's value to the chosen
// variable. Works for both single-record responses (object) and list
// responses (array of objects).
export const applyResponseMapping = ({
  records,
  responseMapping,
  variables,
  logs,
  notFoundPrefix = "Field",
}: {
  records: unknown;
  responseMapping: ResponseMapping | undefined;
  variables: VariableSetter;
  logs: Logger;
  notFoundPrefix?: string;
}) => {
  if (!responseMapping || responseMapping.length === 0) return;

  const list = Array.isArray(records) ? records : [records];
  const first = list[0] as Record<string, unknown> | undefined;

  responseMapping.forEach((mapping) => {
    if (!mapping.fieldName || !mapping.variableId) return;
    if (!first || isNotDefined(first[mapping.fieldName])) {
      logs.add(`${notFoundPrefix} ${mapping.fieldName} does not exist in the response`);
      return;
    }
    const items = list.map(
      (item) => (item as Record<string, unknown>)[mapping.fieldName!],
    );
    variables.set({
      id: mapping.variableId,
      value: items.length === 1 ? items[0] : items,
    });
  });
};
