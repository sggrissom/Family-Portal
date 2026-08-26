import * as preact from "preact";
import * as vlens from "vlens";
import { ProfileImage } from "./ResponsiveImage";
import {
  ModalDialogState,
  attrsModalDialog,
  closeModalDialog,
  newModalDialog,
} from "../lib/modalDialog";
import "./crop-selector-styles";

export interface CropValues {
  cropX: number;
  cropY: number;
  cropScale: number;
}

const MIN_SCALE = 1;
const MAX_SCALE = 3;

interface CropSelectorProps {
  photoId: number;
  crop: CropValues;
  onSave: () => void;
  onCancel: () => void;
}

interface CropEditor {
  crop: CropValues;
  onSave: () => void;
  onCancel: () => void;
  isDragging: boolean;
  dragStartX: number;
  dragStartY: number;
  startCropX: number;
  startCropY: number;
  dialog: ModalDialogState;
}

const useCropEditor = vlens.declareHook((crop: CropValues): CropEditor => {
  const editor: CropEditor = {
    crop,
    onSave: () => {},
    onCancel: () => {},
    isDragging: false,
    dragStartX: 0,
    dragStartY: 0,
    startCropX: 0,
    startCropY: 0,
    dialog: newModalDialog(),
  };
  editor.dialog.onDismiss = () => cancelCrop(editor);
  return editor;
});

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

function onPointerDown(editor: CropEditor, event: PointerEvent) {
  event.preventDefault();
  (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
  editor.isDragging = true;
  editor.dragStartX = event.clientX;
  editor.dragStartY = event.clientY;
  editor.startCropX = editor.crop.cropX;
  editor.startCropY = editor.crop.cropY;
  vlens.scheduleRedraw();
}

function onPointerMove(editor: CropEditor, event: PointerEvent) {
  if (!editor.isDragging) return;

  event.preventDefault();

  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const deltaX = event.clientX - editor.dragStartX;
  const deltaY = event.clientY - editor.dragStartY;

  const sensitivity = 100 / editor.crop.cropScale;
  editor.crop.cropX = clamp(editor.startCropX - (deltaX / rect.width) * sensitivity, 0, 100);
  editor.crop.cropY = clamp(editor.startCropY - (deltaY / rect.height) * sensitivity, 0, 100);
  vlens.scheduleRedraw();
}

function onPointerUp(editor: CropEditor, event: PointerEvent) {
  const container = event.currentTarget as HTMLElement;
  if (container.hasPointerCapture(event.pointerId)) {
    container.releasePointerCapture(event.pointerId);
  }
  editor.isDragging = false;
  vlens.scheduleRedraw();
}

function onCropKeyDown(editor: CropEditor, event: KeyboardEvent) {
  const crop = editor.crop;
  const step = 5 / crop.cropScale;
  switch (event.key) {
    case "ArrowLeft":
      crop.cropX = clamp(crop.cropX - step, 0, 100);
      break;
    case "ArrowRight":
      crop.cropX = clamp(crop.cropX + step, 0, 100);
      break;
    case "ArrowUp":
      crop.cropY = clamp(crop.cropY - step, 0, 100);
      break;
    case "ArrowDown":
      crop.cropY = clamp(crop.cropY + step, 0, 100);
      break;
    case "+":
    case "=":
      crop.cropScale = clamp(crop.cropScale + 0.1, MIN_SCALE, MAX_SCALE);
      break;
    case "-":
      crop.cropScale = clamp(crop.cropScale - 0.1, MIN_SCALE, MAX_SCALE);
      break;
    default:
      return;
  }
  event.preventDefault();
  vlens.scheduleRedraw();
}

function onWheel(editor: CropEditor, event: WheelEvent) {
  event.preventDefault();
  const delta = event.deltaY > 0 ? -0.1 : 0.1;
  editor.crop.cropScale = clamp(editor.crop.cropScale + delta, MIN_SCALE, MAX_SCALE);
  vlens.scheduleRedraw();
}

function onScaleInput(editor: CropEditor, event: Event) {
  const target = event.target as HTMLInputElement;
  editor.crop.cropScale = parseFloat(target.value);
  vlens.scheduleRedraw();
}

function onReset(editor: CropEditor) {
  editor.crop.cropX = 50;
  editor.crop.cropY = 50;
  editor.crop.cropScale = 1;
  vlens.scheduleRedraw();
}

function cancelCrop(editor: CropEditor) {
  closeModalDialog(editor.dialog);
  editor.onCancel();
}

function saveCrop(editor: CropEditor) {
  closeModalDialog(editor.dialog);
  editor.onSave();
}

export const CropSelector = ({ photoId, crop, onSave, onCancel }: CropSelectorProps) => {
  const editor = useCropEditor(crop);
  editor.onSave = onSave;
  editor.onCancel = onCancel;

  const imageSrc = `/api/photo/${photoId}/large`;

  const previewStyle = {
    transform: `scale(${crop.cropScale})`,
    transformOrigin: `${crop.cropX}% ${crop.cropY}%`,
  };

  return (
    <div
      className="crop-selector-modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="cropDialogTitle"
      aria-describedby="cropDialogHint"
      {...attrsModalDialog(editor.dialog)}
    >
      <div className="crop-selector-content">
        <div className="crop-selector-header">
          <h2 id="cropDialogTitle">Adjust Profile Photo</h2>
          <p id="cropDialogHint">
            Drag to pan or use the arrow keys, and the slider to zoom. Escape closes without saving.
          </p>
        </div>

        <div className="crop-selector-body">
          <div
            className={`crop-container ${editor.isDragging ? "dragging" : ""}`}
            role="application"
            aria-label="Crop area — arrow keys pan the photo"
            tabIndex={0}
            onPointerDown={vlens.cachePartial(onPointerDown, editor)}
            onPointerMove={vlens.cachePartial(onPointerMove, editor)}
            onPointerUp={vlens.cachePartial(onPointerUp, editor)}
            onPointerCancel={vlens.cachePartial(onPointerUp, editor)}
            onWheel={vlens.cachePartial(onWheel, editor)}
            onKeyDown={vlens.cachePartial(onCropKeyDown, editor)}
          >
            <div className="crop-image-wrapper" style={previewStyle}>
              <img src={imageSrc} alt="Crop preview" className="crop-image" draggable={false} />
            </div>
            <div className="crop-overlay">
              <div className="crop-circle"></div>
            </div>
          </div>

          <div className="crop-preview-section">
            <h4>Preview</h4>
            <div className="crop-preview-container">
              <ProfileImage
                photoId={photoId}
                alt="Profile photo preview"
                cropX={crop.cropX}
                cropY={crop.cropY}
                cropScale={crop.cropScale}
              />
            </div>
          </div>
        </div>

        <div className="crop-controls">
          <label className="zoom-label">
            <span>Zoom: {crop.cropScale.toFixed(1)}x</span>
            <input
              type="range"
              min={MIN_SCALE}
              max={MAX_SCALE}
              step="0.1"
              value={crop.cropScale}
              onInput={vlens.cachePartial(onScaleInput, editor)}
              className="zoom-slider"
            />
          </label>
          <button
            type="button"
            className="btn btn-outline btn-sm"
            onClick={vlens.cachePartial(onReset, editor)}
          >
            Reset
          </button>
        </div>

        <div className="crop-selector-actions">
          <button
            type="button"
            className="btn btn-outline"
            onClick={vlens.cachePartial(cancelCrop, editor)}
          >
            Cancel
          </button>
          <button
            type="button"
            className="btn btn-primary"
            onClick={vlens.cachePartial(saveCrop, editor)}
          >
            Save as Profile Photo
          </button>
        </div>
      </div>
    </div>
  );
};
