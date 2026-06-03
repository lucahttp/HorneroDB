// Mirrors the helper from the official nocodb forge block.
// Drops null/undefined values and stringifies the rest.
export const parseSearchParams = (
  records: Record<string, unknown>,
): Record<string, string> => {
  return Object.entries(records).reduce<Record<string, string>>(
    (acc, [key, value]) => {
      if (value === null || value === undefined) return acc;
      acc[key] = value.toString();
      return acc;
    },
    {},
  );
};
