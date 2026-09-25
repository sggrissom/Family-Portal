import * as preact from "preact";
import type * as server from "../server";
import { faceCropLayout, hasFaceBox } from "../lib/faceCrop";
import "./face-crop-styles";

interface FaceCropProps {
  photoId: number;
  box: server.FaceBox;
  size?: number;
  alt?: string;
  className?: string;
}

export const FaceCrop = ({ photoId, box, size = 88, alt = "Face", className }: FaceCropProps) => {
  const frameClass = `face-crop ${className || ""}`.trim();
  if (!hasFaceBox(box)) {
    return (
      <div className={frameClass} style={{ width: size, height: size }}>
        <img
          src={`/api/photo/${photoId}/thumb`}
          alt={alt}
          loading="lazy"
          className="face-crop-whole"
        />
      </div>
    );
  }
  return (
    <div className={frameClass} style={{ width: size, height: size }}>
      <img src={`/api/photo/${photoId}/medium`} alt={alt} style={faceCropLayout(box)} />
    </div>
  );
};
