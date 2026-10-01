<template>
  <v-app>
    <v-toolbar density="compact" :elevation="1">
      <span class="ml-3 text-title-large">
        <router-link to="/" :style="`color: ${theme.global.current.value.dark ? 'white' : 'black'}; text-decoration: none`">
          <v-icon start icon="mdi-graph-outline" />
          TAF Web UI
        </router-link>
      </span>

      <v-btn-toggle class="ml-4">
          <v-btn to="/tmis" title="TMIs">
            <v-icon start>mdi-rhombus-outline</v-icon>
            <span class="hidden-sm-and-down">TMIs</span>
          </v-btn>

          <v-btn to="/sessions" title="Sessions">
            <v-icon start>mdi-folder-account-outline</v-icon>
            <span class="hidden-sm-and-down">Sessions</span>
          </v-btn>
      </v-btn-toggle>

      <v-chip :prepend-icon="connectionState === 'connected' ? 'mdi-ethernet-cable' : 'mdi-ethernet-cable-off'" class="ml-2" size="small" label title="WebSocket State" :color="connectionState === 'connected' ? 'success' : 'warning'">
        {{ connectionState }}
      </v-chip>

      <v-spacer />
      <div id="toolbar" class="d-flex flex-grow-1 justify-end">
      </div>

      <v-btn icon title="Toggle Theme" size="small" @click="toggleTheme()" variant="text">
        <v-icon icon="mdi-theme-light-dark" />
      </v-btn>
    </v-toolbar>
    <v-main>
        <v-skeleton-loader type="paragraph@10" v-if="store.loading" />
        <alert-dialog ref="dialog" />
        <router-view />
    </v-main>
  </v-app>
</template>

<script lang="ts" setup>
import { useTheme } from 'vuetify';
import { ref, provide } from 'vue';
import { AlertOptions } from '@/types';

import { Dialog } from '@/symbols';
import { useAppStore } from '@/stores/app';
import AlertDialog from './components/AlertDialog.vue';

const dialog = ref<InstanceType<typeof AlertDialog> | null>(null);

let connectionState = ref('');
let hasConnected = false;

function dialogCallHelper(fn: 'alert' | 'confirm' | 'prompt' | 'select', title: string, message: string = '', options: AlertOptions = {}): Promise<any> {
  if (!dialog.value) {
    return Promise.reject();
  } else {
    return dialog.value[fn](title, message, options);
  }
}

provide(Dialog, {
  alert(title: string, message: string, options: AlertOptions): Promise<any> {
    return dialogCallHelper('alert', title, message, options);
  },
  prompt: function (title: string, message: string, options: AlertOptions): Promise<any> {
    return dialogCallHelper('prompt', title, message, options);
  },
  select: function (title: string, message: string, options: AlertOptions): Promise<any> {
    return dialogCallHelper('select', title, message, options);
  },
  confirm: function (title: string, message: string, options: AlertOptions): Promise<any> {
    return dialogCallHelper('confirm', title, message, options);
  }
});

const store = useAppStore();

// messages are applied in batches, so that the UI is re-rendered at most once per interval instead of once per message
const MESSAGE_BATCH_INTERVAL = 100;
let queuedMessages: string[] = [];
let flushTimer: ReturnType<typeof setTimeout> | null = null;

function flushMessages() {
  flushTimer = null;
  const batch = queuedMessages;
  queuedMessages = [];
  const msgs = [];
  for (const data of batch) {
    try {
      msgs.push(JSON.parse(data));
    } catch (e) {
      console.log('[ws] error', e);
    }
  }
  store.processMessages(msgs);
}

function connect() {
  const ws = new WebSocket(`${location.origin.replace(/^http/, 'ws')}/ws`);

  connectionState.value = 'connecting';

  ws.addEventListener('open', () => {
    console.log('[ws] connected');
    connectionState.value = 'connected';
    store.setSocket(ws);
    // events may have been missed while disconnected, so fetch the current state again
    if (hasConnected) {
      store.resync();
    }
    hasConnected = true;
  });
  ws.addEventListener('close', () => {
    console.log('[ws] closed, reconnect in 1 sec');
    connectionState.value = 'reconnecting';
    setTimeout(() => connect(), 1000);
    store.setSocket(null);
  });
  ws.addEventListener('error', (error) => {
    console.log('[ws] error', error);
    connectionState.value = 'error';
    ws.close();
  });
  ws.addEventListener('message', (evt) => {
    queuedMessages.push(evt.data);
    if (flushTimer === null) {
      flushTimer = setTimeout(flushMessages, MESSAGE_BATCH_INTERVAL);
    }
  });
}

connect();
store.init();

const theme = useTheme();

function toggleTheme() {
  theme.global.name.value = theme.global.current.value.dark ? 'light' : 'dark';
  localStorage.setItem('theme', theme.global.name.value);
}
</script>
