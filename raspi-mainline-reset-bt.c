// +build ignore
// The line above is required otherwise Go thinks this is part of the build

#define BUS "serial"
#define DRIVER "hci_uart_bcm"
#define DEVICE "serial0-0"

#define DEVICE_LEN sizeof(DEVICE)

#include <stdio.h>
#include <stdlib.h>
#include <stdbool.h>


int bind_unbind_device(bool bind) {
  char *bind_str;
  if (bind)
    bind_str = "bind";
  else
    bind_str = "unbind";

  char formatted_str[100];

  sprintf(formatted_str, "/sys/bus/"BUS"/drivers/"DRIVER"/%s", bind_str);
  FILE *f = fopen(formatted_str, "w");
  if (f == NULL) {
    sprintf(formatted_str, "Unable to open "DRIVER" driver %s file", bind_str);
    perror(formatted_str);
    return EXIT_FAILURE;
  }

  size_t bytes = fwrite(DEVICE"\n", DEVICE_LEN + 1, 1, f);
  if (bytes < 1) {
    sprintf(formatted_str, "Unable to %s "DEVICE" %s driver "DRIVER, bind_str, bind ? "to" : "from");
    perror(formatted_str);
    return EXIT_FAILURE;
  }

  int ret = fflush(f);
  if (ret == EOF) {
    sprintf(formatted_str, "Unable to %s "DEVICE" %s driver "DRIVER, bind_str, bind ? "to" : "from");
    perror(formatted_str);
    return EXIT_FAILURE;
  }

  ret = fclose(f);
  if (ret == EOF) {
    sprintf(formatted_str, "Unable to close "DRIVER" driver %s file", bind_str);
    perror(formatted_str);
    return EXIT_FAILURE;
  }

  return EXIT_SUCCESS;
}

int main() {
  printf("Resetting Raspberry Pi bluetooth...\n");

  // Unbind
  int ret = bind_unbind_device(0);
  if (ret != EXIT_SUCCESS) {
    fprintf(stderr, "Unable to unbind, maybe it's already unbound?\n");
  }

  // Bind
  ret = bind_unbind_device(1);
  if (ret != EXIT_SUCCESS) {
    return ret;
  }

  // Restart Bluez
  printf("Restarting Bluez...\n");
  ret = system("systemctl restart bluetooth.service");
  if (ret < 0) {
    perror("Failed to call systemctl to restart Bluez: ");
    return EXIT_FAILURE;
  } else if (ret > 0) {
    fprintf(stderr, "systemctl returned non-zero exit status %d\n", ret);
    return ret;
  }

  return EXIT_SUCCESS;
}
