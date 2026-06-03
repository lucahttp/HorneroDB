// Converts a list of { key, value } entries into a plain object body that
// HorneroDB's record endpoints accept. Empty keys/values are dropped on create
// and explicit `null` is sent on update when the value is empty so columns
// can be cleared.
export const parseRecordCreateBody = (
  fields: { key?: string; value?: string }[],
): Record<string, unknown> => {
  const record: Record<string, unknown> = {};
  fields.forEach(({ key, value }) => {
    if (!key || !value) return;
    record[key] = value;
  });
  return record;
};

export const parseRecordUpdateBody = (
  updates: { key?: string; value?: string }[],
): Record<string, unknown> => {
  const record: Record<string, unknown> = {};
  updates.forEach(({ key, value }) => {
    if (!key) return;
    record[key] = value ?? null;
  });
  return record;
};
