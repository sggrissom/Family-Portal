import * as preact from "preact";
import type * as server from "../server";
import { faceCropLayout, hasFaceBox } from "../lib/faceCrop";
import "./face-crop-styles";

interface FaceCropProps {
  photoId: number;
  box: server.FaceBox;
  size?: number;
  fluid?: boolean;
  alt?: string;
  className?: string;
}

export const FaceCrop = ({
  photoId,
  box,
  size = 88,
  fluid = false,
  alt = "Face",
  className,
}: FaceCropProps) => {
  const frameClass = `face-crop ${fluid ? "face-crop-fluid" : ""} ${className || ""}`.trim();
  const frameStyle = fluid ? undefined : { width: size, height: size };
  if (!hasFaceBox(box)) {
    return (
      <div className={frameClass} style={frameStyle}>
        <img
          src={`/api/photo/${photoId}/${fluid ? "medium" : "thumb"}`}
          alt={alt}
          loading="lazy"
          className="face-crop-whole"
        />
      </div>
    );
  }
  return (
    <div className={frameClass} style={frameStyle}>
      <img src={`/api/photo/${photoId}/medium`} alt={alt} style={faceCropLayout(box)} />
    </div>
  );
};
