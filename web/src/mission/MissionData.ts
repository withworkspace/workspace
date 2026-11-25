import { createContext } from "react";
import type { MissionDetail } from "../api";

// MissionData is the mission a page is about, for what draws a ref (a crew's
// badge) to say more about it on hover.
export const MissionData = createContext<MissionDetail | undefined>(undefined);
