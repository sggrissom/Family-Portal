import * as preact from "preact";
import { Play } from "lucide-preact";
import * as server from "../server";
import { Icon } from "./Icon";
import { formatDuration } from "../lib/videoFiles";
import "./video-styles";

export const VideoBadge = ({ image }: { image: server.Image | undefined }) => {
  if (image?.mediaKind !== server.MediaVideo) return null;
  return (
    <span className="video-badge">
      <Icon symbol={Play} size={12} label="Video" />
      {image.durationMs > 0 && formatDuration(image.durationMs)}
    </span>
  );
};
