"""
Custom logging that looks nice.

Thanks to the Tornado web framework for much of this code.
"""
import sys
import time
import logging
import threading

try:
    import curses
except ImportError:
    curses = None


class ThreadInfoFilter(logging.Filter):
    def filter(self, record):
        record.thread_name = threading.current_thread().name
        return record


def _safe_unicode(s):
    try:
        return unicode(s)
    except UnicodeDecodeError:
        return repr(s)


def _stderr_supports_color():
    color = False
    if curses and sys.stderr.isatty():
        try:
            curses.setupterm()
            if curses.tigetnum("colors") > 0:
                color = True
        except Exception:
            pass
    return color


class CustomFormatter(logging.Formatter):
    def __init__(self, *args, **kwargs):
        logging.Formatter.__init__(self, *args, **kwargs)
        self._color = _stderr_supports_color()
        if self._color:
            fg_color = (curses.tigetstr("setaf") or
                        curses.tigetstr("setf") or "")
            if (3, 0) < sys.version_info < (3, 2, 3):
                fg_color = unicode(fg_color, "ascii")
            self._colors = {
                logging.DEBUG: unicode(curses.tparm(fg_color, 4),   # Blue
                                       "ascii"),
                logging.INFO: unicode(curses.tparm(fg_color, 2),    # Green
                                      "ascii"),
                logging.WARNING: unicode(curses.tparm(fg_color, 3), # Yellow
                                         "ascii"),
                logging.ERROR: unicode(curses.tparm(fg_color, 1),   # Red
                                       "ascii"),
                logging.FATAL: unicode(curses.tparm(fg_color, 1),   # Red
                                       "ascii"),
            }
            self._normal = unicode(curses.tigetstr("sgr0"), "ascii")


    def format(self, record):
        try:
            record.message = record.getMessage()
        except Exception as e:
            record.message = "Bad message (%r): %r" % (e, record.__dict__)
        assert isinstance(record.message, basestring)  # guaranteed by logging

        # Use a different format for times.
        record.asctime = time.strftime(
            "%y/%m/%d %H:%M:%S", self.converter(record.created))

        # The actual logging format (prefix)
        prefix = '[%(levelname)1.1s %(asctime)s %(thread_name)s %(module)s:%(lineno)d]' % \
            record.__dict__

        # Colorize prefix, if we support it.
        if self._color:
            prefix = (self._colors.get(record.levelno, self._normal) +
                      prefix + self._normal)

        formatted = prefix + " " + _safe_unicode(record.message)

        # Create exception information.
        if record.exc_info:
            if not record.exc_text:
                record.exc_text = self.formatException(record.exc_info)

        if record.exc_text:
            lines = [formatted.rstrip()]
            lines.extend(_safe_unicode(ln) for ln in record.exc_text.split('\n'))
            formatted = '\n'.join(lines)

        # Don't print a blank line between log entries that already have
        # newlines.
        if '\n' in formatted and formatted.endswith('\n'):
            formatted = formatted.rstrip("\r\n")

        return formatted.replace("\n", "\n    ")


def setup_logging(level=None):
    if level is None:
        level = logging.INFO

    log = logging.getLogger()

    log.setLevel(level)
    stream = logging.StreamHandler()
    stream.setFormatter(CustomFormatter())
    stream.addFilter(ThreadInfoFilter())
    log.addHandler(stream)
