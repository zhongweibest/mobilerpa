import { defineStore } from "pinia";
import { ref } from "vue";

export type NoticeType = "success" | "warning" | "error" | "info";

export interface NoticeItem {
  id: number;
  type: NoticeType;
  message: string;
}

const MAX_NOTICES = 3;

export const useNoticesStore = defineStore("notices", () => {
  const notices = ref<NoticeItem[]>([]);
  const timers = new Map<number, number>();
  let nextID = 1;

  function removeNotice(id: number) {
    notices.value = notices.value.filter((item) => item.id !== id);
    const timer = timers.get(id);
    if (timer) {
      window.clearTimeout(timer);
      timers.delete(id);
    }
  }

  function pushNotice(type: NoticeType, message: string, duration = 5000) {
    const trimmed = message.trim();
    if (trimmed === "") {
      return;
    }

    const duplicate = notices.value.find((item) => item.type === type && item.message === trimmed);
    if (duplicate) {
      const timer = timers.get(duplicate.id);
      if (timer) {
        window.clearTimeout(timer);
      }
      if (duration > 0) {
        timers.set(
          duplicate.id,
          window.setTimeout(() => {
            removeNotice(duplicate.id);
          }, duration)
        );
      }
      return;
    }

    const notice: NoticeItem = {
      id: nextID++,
      type,
      message: trimmed
    };

    notices.value = [notice, ...notices.value].slice(0, MAX_NOTICES);
    while (notices.value.length >= MAX_NOTICES && notices.value.some((item) => item.id !== notice.id) && notices.value.length > MAX_NOTICES - 1) {
      const removed = notices.value.pop();
      if (removed) {
        removeNotice(removed.id);
      }
    }

    if (duration > 0) {
      timers.set(
        notice.id,
        window.setTimeout(() => {
          removeNotice(notice.id);
        }, duration)
      );
    }
  }

  function error(message: string, duration = 5000) {
    pushNotice("error", message, duration);
  }

  function warning(message: string, duration = 5000) {
    pushNotice("warning", message, duration);
  }

  function success(message: string, duration = 3000) {
    pushNotice("success", message, duration);
  }

  function info(message: string, duration = 4000) {
    pushNotice("info", message, duration);
  }

  return {
    notices,
    pushNotice,
    removeNotice,
    error,
    warning,
    success,
    info
  };
});
