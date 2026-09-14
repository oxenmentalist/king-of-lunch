#include <stdint.h>
void kol_run(const char *controls);
void kol_show(uint64_t identifier, const char *path);
void kol_anchor(uint64_t identifier, const char *fragment);
void kol_begin(uint64_t identifier, uint64_t generation);
void kol_content(uint64_t identifier, uint64_t generation, const char *html);
void kol_error(uint64_t identifier, uint64_t generation, const char *message);
void kol_zoom(uint64_t identifier, int size);
char *kol_launch(const char *path, const char *application);
void kol_evaluate(uint64_t identifier, uint64_t token, const char *script);
void kol_action(uint64_t identifier, const char *action);
void kol_resize(uint64_t identifier, int width, int height);
void kol_stop(void);
