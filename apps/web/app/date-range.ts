import type { FormEvent } from "react";

/** Reset the editable end date whenever the start date changes. */
export function syncDateRange(
  event: FormEvent<HTMLFormElement>,
  endName = "end_date",
) {
  const start = event.currentTarget.elements.namedItem("start_date");
  const end = event.currentTarget.elements.namedItem(endName);
  if (!(start instanceof HTMLInputElement) || !(end instanceof HTMLInputElement)) return;

  end.min = start.value;
  if (event.type === "change" && event.target === start) {
    end.value = start.value;
  }
}
