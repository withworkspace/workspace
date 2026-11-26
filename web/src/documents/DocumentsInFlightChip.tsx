import { Chip } from "../ui/Chip";

// DocumentsInFlightChip says a document is changed in capsules: in flight.
export function DocumentsInFlightChip({ capsules }: { capsules: string[] }) {
  return (
    <Chip tone="progress" title={`Changed in ${capsules.join(", ")}, not landed`}>
      in flight
    </Chip>
  );
}
