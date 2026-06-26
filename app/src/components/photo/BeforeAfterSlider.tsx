import React, { useCallback, useEffect, useRef, useState } from "react";
import { cn } from "@/components/ui/lib/utils.ts";

interface Props {
  before: string;
  after: string;
  className?: string;
}

// before/after 对比滑块：底层显示结果图（after），顶层用可拖动分割线揭示原图（before）。
// 同时支持鼠标与触摸（pointer 事件）。
const BeforeAfterSlider: React.FC<Props> = ({ before, after, className }) => {
  const ref = useRef<HTMLDivElement>(null);
  const dragging = useRef(false);
  const [pos, setPos] = useState(50); // 分割线位置（百分比）
  const [width, setWidth] = useState(0); // 容器像素宽，用于让 before 图与 after 图等宽

  const update = useCallback((clientX: number) => {
    const el = ref.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const p = ((clientX - rect.left) / rect.width) * 100;
    setPos(Math.max(0, Math.min(100, p)));
  }, []);

  // 容器宽度跟随尺寸变化
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setWidth(el.clientWidth));
    ro.observe(el);
    setWidth(el.clientWidth);
    return () => ro.disconnect();
  }, []);

  // 拖动期间监听全局 pointer 事件，松手即停
  useEffect(() => {
    const move = (e: PointerEvent) => {
      if (dragging.current) update(e.clientX);
    };
    const up = () => {
      dragging.current = false;
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
    return () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
    };
  }, [update]);

  return (
    <div
      ref={ref}
      className={cn("relative select-none overflow-hidden rounded-md bg-muted touch-none", className)}
      onPointerDown={(e) => {
        dragging.current = true;
        update(e.clientX);
      }}
    >
      {/* after（结果）：定义容器尺寸 */}
      <img src={after} alt="after" draggable={false} className="block w-full select-none object-contain pointer-events-none" />

      {/* before（原图）：被分割线裁切，宽度锁定为容器宽以与 after 对齐 */}
      <div className="absolute inset-y-0 left-0 overflow-hidden pointer-events-none" style={{ width: `${pos}%` }}>
        <img
          src={before}
          alt="before"
          draggable={false}
          className="block h-full max-w-none select-none object-contain pointer-events-none"
          style={{ width: width || "100%" }}
        />
      </div>

      {/* 分割线与拖动手柄 */}
      <div className="absolute inset-y-0 z-10 w-0.5 -ml-px bg-white shadow" style={{ left: `${pos}%` }}>
        <div className="absolute top-1/2 left-1/2 flex h-7 w-7 -translate-x-1/2 -translate-y-1/2 cursor-ew-resize items-center justify-center rounded-full bg-white text-[11px] text-black shadow">
          ⇆
        </div>
      </div>

      {/* 角标 */}
      <span className="absolute left-1 top-1 rounded bg-black/60 px-1.5 py-0.5 text-[10px] text-white">before</span>
      <span className="absolute right-1 top-1 rounded bg-black/60 px-1.5 py-0.5 text-[10px] text-white">after</span>
    </div>
  );
};

export default BeforeAfterSlider;
